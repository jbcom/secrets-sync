set shell := ["bash", "-euo", "pipefail", "-c"]

gopy_version := "v0.4.10"
x_tools_version := "v0.47.0"
gomarkdoc_version := "v1.1.0"
macosx_deployment_target := "11.0"
auditwheel_version := "6.7.0"

default:
    @just --list

# Run the full local CI surface.
ci python_version="3.13":
    just test-go
    just build-all
    just python-build "{{ python_version }}"
    just quality

# Run Go vulnerability scanning.
vuln:
    #!/usr/bin/env bash
    set -euo pipefail
    packages_raw="$(scripts/go-packages.sh)"
    [[ -n "${packages_raw}" ]] || { echo "no Go packages discovered" >&2; exit 1; }
    mapfile -t packages <<<"${packages_raw}"
    GOTOOLCHAIN="${GO_TOOLCHAIN:-go1.26.4}" go run golang.org/x/vuln/cmd/govulncheck@v1.3.0 "${packages[@]}"

# Run Go tests. Extra arguments are passed to go test.
test-go *args:
    #!/usr/bin/env bash
    set -euo pipefail
    packages_raw="$(scripts/go-packages.sh)"
    [[ -n "${packages_raw}" ]] || { echo "no Go packages discovered" >&2; exit 1; }
    mapfile -t packages <<<"${packages_raw}"
    GOTOOLCHAIN="${GO_TOOLCHAIN:-go1.26.4}" go test "${packages[@]}" {{ args }}

# Run Go tests with the race detector and coverage.
test-unit:
    #!/usr/bin/env bash
    set -euo pipefail
    packages_raw="$(scripts/go-packages.sh)"
    [[ -n "${packages_raw}" ]] || { echo "no Go packages discovered" >&2; exit 1; }
    mapfile -t packages <<<"${packages_raw}"
    GOTOOLCHAIN="${GO_TOOLCHAIN:-go1.26.4}" go test -race -coverprofile=coverage.out "${packages[@]}"

# Build all Go release binaries used by local workflows.
build-all: build controller-build lambda-build

# Build the Go CLI.
build:
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p bin
    version="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
    commit="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo none)}"
    date="${DATE:-$(date -u '+%Y-%m-%dT%H:%M:%SZ')}"
    GOTOOLCHAIN="${GO_TOOLCHAIN:-go1.26.4}" go build \
      -ldflags "-s -w -X github.com/jbcom/secrets-sync/cmd/secrets-sync/cmd.Version=${version} -X github.com/jbcom/secrets-sync/cmd/secrets-sync/cmd.Commit=${commit} -X github.com/jbcom/secrets-sync/cmd/secrets-sync/cmd.Date=${date}" \
      -o bin/secrets-sync ./cmd/secrets-sync

# Build the Kubernetes controller binary.
controller-build:
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p bin
    version="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
    commit="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo none)}"
    date="${DATE:-$(date -u '+%Y-%m-%dT%H:%M:%SZ')}"
    GOTOOLCHAIN="${GO_TOOLCHAIN:-go1.26.4}" go build -trimpath \
      -ldflags "-s -w -X github.com/jbcom/secrets-sync/cmd/secrets-sync/cmd.Version=${version} -X github.com/jbcom/secrets-sync/cmd/secrets-sync/cmd.Commit=${commit} -X github.com/jbcom/secrets-sync/cmd/secrets-sync/cmd.Date=${date}" \
      -o bin/secrets-sync-controller ./cmd/secrets-sync-controller

# Build the AWS Lambda bootstrap binary.
lambda-build:
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p dist/lambda
    version="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
    commit="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo none)}"
    date="${DATE:-$(date -u '+%Y-%m-%dT%H:%M:%SZ')}"
    GOTOOLCHAIN="${GO_TOOLCHAIN:-go1.26.4}" GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X github.com/jbcom/secrets-sync/cmd/secrets-sync/cmd.Version=${version} -X github.com/jbcom/secrets-sync/cmd/secrets-sync/cmd.Commit=${commit} -X github.com/jbcom/secrets-sync/cmd/secrets-sync/cmd.Date=${date}" \
      -o dist/lambda/bootstrap ./cmd/secrets-sync-lambda

# Install patched gopy and goimports into .tools/bin.
python-tools:
    mkdir -p .tools/bin
    GOTOOLCHAIN="${GO_TOOLCHAIN:-go1.26.4}" GOBIN="$PWD/.tools/bin" GOPY_VERSION="{{ gopy_version }}" X_TOOLS_VERSION="{{ x_tools_version }}" bash scripts/install-gopy.sh

# Generate the gopy binding package.
python-bindings python_version="3.13": python-tools
    #!/usr/bin/env bash
    set -euo pipefail
    python_pkg="secrets_sync"
    python_dist="secrets-sync-python-binding"
    output="python/build/${python_pkg}"
    python_run=(uv run --no-project --python "{{ python_version }}" --with build --with pybindgen --with setuptools --with wheel --)
    binding_version="$(scripts/python-binding-version.sh)"

    if [[ "$(uname -s)" == "Darwin" ]]; then
      export MACOSX_DEPLOYMENT_TARGET="${MACOSX_DEPLOYMENT_TARGET:-{{ macosx_deployment_target }}}"
    fi

    echo "Generating Python bindings for Python {{ python_version }}..."
    rm -rf "${output}"
    mkdir -p "${output}"

    gopy_env="$("${python_run[@]}" python - <<'PY'
    import os
    from pathlib import Path
    import sys
    import sysconfig

    def existing(candidates, description):
        for value in candidates:
            if not value or str(value).startswith("/install/"):
                continue
            path = Path(value)
            if path.exists():
                return os.fspath(path)
        raise SystemExit(f"could not find {description}: {candidates!r}")

    version = f"{sys.version_info.major}.{sys.version_info.minor}"
    abiflags = getattr(sys, "abiflags", "")
    prefix = Path(sys.base_prefix)
    include = existing(
        [
            sysconfig.get_path("include"),
            prefix / "include" / f"python{version}{abiflags}",
            prefix / "include" / f"python{version}",
        ],
        "Python include directory",
    )
    libdir = existing(
        [
            sysconfig.get_config_var("LIBDIR"),
            prefix / "lib",
        ],
        "Python library directory",
    )
    libpy = (
        sysconfig.get_config_var("LDLIBRARY")
        or sysconfig.get_config_var("LIBRARY")
        or f"libpython{version}{abiflags}.dylib"
    )
    for prefix_text in ("lib",):
        if libpy.startswith(prefix_text):
            libpy = libpy[len(prefix_text):]
    for suffix in (".dylib", ".so", ".a"):
        if libpy.endswith(suffix):
            libpy = libpy[:-len(suffix)]

    print(f"GOPY_INCLUDE={include}")
    print(f"GOPY_LIBDIR={libdir}")
    print(f"GOPY_PYLIB={libpy}")
    PY
    )"

    env ${gopy_env} PATH="$PWD/.tools/bin:$PATH" \
      "${python_run[@]}" "$PWD/.tools/bin/gopy" pkg \
      -output="${output}" \
      -name="${python_pkg}" \
      -author="jbcom" \
      -email="support@extended-data.dev" \
      -url="https://github.com/jbcom/secrets-sync" \
      -desc="Enterprise-grade secret synchronization pipeline with Python bindings" \
      "github.com/jbcom/secrets-sync/python/${python_pkg}"

    "${python_run[@]}" python tools/patch_python_dist.py --package-dir "${output}" --name "${python_dist}" --version "${binding_version}"
    echo "Python bindings generated in ${output}"

# Build and smoke-test the gopy wheel for one Python version.
python-build python_version="3.13":
    #!/usr/bin/env bash
    set -euo pipefail
    python_pkg="secrets_sync"
    python_dist="secrets-sync-python-binding"
    output="python/build/${python_pkg}"
    python_run=(uv run --no-project --python "{{ python_version }}" --with build --with pybindgen --with setuptools --with wheel --)
    binding_version="$(scripts/python-binding-version.sh)"

    if [[ "$(uname -s)" == "Darwin" ]]; then
      export MACOSX_DEPLOYMENT_TARGET="${MACOSX_DEPLOYMENT_TARGET:-{{ macosx_deployment_target }}}"
    fi

    just python-bindings "{{ python_version }}"
    echo "Building Python wheel for Python {{ python_version }}..."
    (cd "${output}" && "${python_run[@]}" python -m build)
    just python-repair-wheels "{{ python_version }}"
    "${python_run[@]}" python tools/check_python_dist.py --dist-dir "${output}/dist" --name "${python_dist}" --version "${binding_version}"
    PYTHON_VERSION="{{ python_version }}" PYTHON_DIST_DIR="${output}/dist" bash scripts/check-python-binding.sh

# Repair Linux gopy wheels to PyPI-accepted manylinux tags.
python-repair-wheels python_version="3.13":
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ "$(uname -s)" != "Linux" ]]; then
      exit 0
    fi

    dist_dir="python/build/secrets_sync/dist"
    shopt -s nullglob
    wheels=("${dist_dir}"/*linux_*.whl)
    if [[ "${#wheels[@]}" -eq 0 ]]; then
      exit 0
    fi

    repaired_dir="${dist_dir}/repaired"
    rm -rf "${repaired_dir}"
    mkdir -p "${repaired_dir}"

    uv run --no-project --python "{{ python_version }}" --with "auditwheel=={{ auditwheel_version }}" -- \
      auditwheel repair -w "${repaired_dir}" "${wheels[@]}"

    rm -f "${wheels[@]}"
    find "${repaired_dir}" -maxdepth 1 -type f -name '*.whl' -exec mv {} "${dist_dir}/" \;
    rmdir "${repaired_dir}"

# Build and smoke-test the Python binding for the supported matrix.
python-matrix:
    #!/usr/bin/env bash
    set -euo pipefail
    for version in 3.11 3.12 3.13 3.14; do
      just python-build "${version}"
      just python-clean
    done

# Verify the latest generated Python wheel metadata.
python-check-dist python_version="3.13":
    #!/usr/bin/env bash
    set -euo pipefail
    binding_version="$(scripts/python-binding-version.sh)"
    uv run --no-project --python "{{ python_version }}" --with build --with pybindgen --with setuptools --with wheel -- \
      python tools/check_python_dist.py --dist-dir python/build/secrets_sync/dist --name secrets-sync-python-binding --version "${binding_version}"

# Install the generated wheel into the active Python environment.
python-install python_version="3.13":
    just python-build "{{ python_version }}"
    uv run --no-project --python "{{ python_version }}" -- \
      python -m pip install --force-reinstall python/build/secrets_sync/dist/*.whl

# Clean generated Python binding output.
python-clean:
    rm -rf python/build

# Generate API docs.
docs-api:
    GOTOOLCHAIN="${GO_TOOLCHAIN:-go1.26.4}" GOMARKDOC_VERSION="{{ gomarkdoc_version }}" bash scripts/generate-api-docs.sh

# Build Sphinx docs with warnings treated as errors.
docs: docs-api
    tox -e docs

# Run lint and docs checks.
quality:
    tox -e lint,pytools,docs

# Run Go formatting.
fmt:
    #!/usr/bin/env bash
    set -euo pipefail
    packages_raw="$(scripts/go-packages.sh)"
    [[ -n "${packages_raw}" ]] || { echo "no Go packages discovered" >&2; exit 1; }
    mapfile -t packages <<<"${packages_raw}"
    GOTOOLCHAIN="${GO_TOOLCHAIN:-go1.26.4}" go fmt "${packages[@]}"

# Run dependency cleanup.
tidy:
    GOTOOLCHAIN="${GO_TOOLCHAIN:-go1.26.4}" go mod tidy

# Download and tidy dependencies.
deps:
    GOTOOLCHAIN="${GO_TOOLCHAIN:-go1.26.4}" go mod download
    GOTOOLCHAIN="${GO_TOOLCHAIN:-go1.26.4}" go mod tidy

# Build the distroless image.
docker-build tag="ghcr.io/jbcom/secrets-sync:ci":
    docker build --build-arg VERSION=ci -t "{{ tag }}" .

# Validate the GoReleaser config.
goreleaser-check:
    GOTOOLCHAIN="${GO_TOOLCHAIN:-go1.26.4}" go run github.com/goreleaser/goreleaser/v2@v2.16.0 check

# Run integration tests. Requires LocalStack + Vault or docker compose.
test-integration:
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ -z "${VAULT_ADDR:-}" || -z "${AWS_ENDPOINT_URL:-}" ]]; then
      docker-compose -f docker-compose.test.yml up --abort-on-container-exit --exit-code-from test-runner
    else
      GOTOOLCHAIN="${GO_TOOLCHAIN:-go1.26.4}" go test -v -tags=integration ./tests/integration/...
    fi

# Run integration tests via docker compose.
test-integration-docker:
    docker-compose -f docker-compose.test.yml down -v 2>/dev/null || true
    docker-compose -f docker-compose.test.yml up --build --abort-on-container-exit --exit-code-from test-runner
    docker-compose -f docker-compose.test.yml down -v

# Start the integration test dependencies.
test-env-up:
    #!/usr/bin/env bash
    set -euo pipefail
    docker-compose -f docker-compose.test.yml up -d localstack vault
    echo "Waiting for services to be healthy..."
    for i in {1..12}; do
      container_ids="$(docker-compose -f docker-compose.test.yml ps -q localstack vault)"
      healthy_count="$(docker inspect -f '{{ "{{" }}if .State.Health{{ "}}" }}{{ "{{" }}.State.Health.Status{{ "}}" }}{{ "{{" }}else{{ "}}" }}unknown{{ "{{" }}end{{ "}}" }}' ${container_ids} 2>/dev/null | grep -c '^healthy$' || true)"
      if [[ "${healthy_count}" -eq 2 ]]; then
        echo "Services are healthy."
        break
      fi
      if [[ "${i}" -eq 12 ]]; then
        echo "Warning: Services may not be fully healthy, proceeding anyway"
      else
        echo "Waiting... (${i}/12)"
        sleep 5
      fi
    done
    cat <<'EOF'
    Test environment ready. Export these variables:
      export VAULT_ADDR=http://localhost:8200
      export VAULT_TOKEN=test-root-token
      export AWS_ENDPOINT_URL=http://localhost:4566
      export AWS_ACCESS_KEY_ID=test
      export AWS_SECRET_ACCESS_KEY=test
      export AWS_REGION=us-east-1

    Then run: just test-integration
    EOF

# Stop the integration test dependencies.
test-env-down:
    docker-compose -f docker-compose.test.yml down -v

# Clean local build artifacts.
clean: python-clean
    rm -f secrets-sync coverage.out
    rm -rf bin dist docs/_build .tools .tox
    docker-compose -f docker-compose.test.yml down -v 2>/dev/null || true
