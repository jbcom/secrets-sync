# SecretSync Handoff

This repo is pre-launch. Prefer the intended public contract over old local
spellings when they conflict.

## Canonical Names

- Product, CLI, binary, GHCR image, Helm chart, Lambda archive, CRD schema,
  controller, and Go command path:
  `secrets-sync`
- Python import/module: `secrets_sync`
- Python PyPI distribution: `secrets-sync-python-binding`
- Metrics namespace: `secrets_sync`
- Repo-owned Python entry point: `secrets_sync`; downstream facades may wrap it.

## Current Source Shape

- Go CLI path: `cmd/secrets-sync`
- Go Lambda path: `cmd/secrets-sync-lambda`
- Go Kubernetes controller path: `cmd/secrets-sync-controller`
- Go binary target: `bin/secrets-sync`
- Kubernetes controller binary target: `bin/secrets-sync-controller`
- Lambda binary target: `dist/lambda/bootstrap`
- gopy binding source: `python/secrets_sync/secrets_sync.go`
- generated binding output: `python/build/secrets_sync`
- Helm chart path: `deploy/charts/secrets-sync`
- Kubernetes CRD path: `deploy/crds/secrets-sync.jbcom.dev_credentialsynchronizations.yaml`
- Kubernetes controller manifests: `deploy/controller`
- Lambda deployment template: `deploy/lambda/template.yaml`
- PyPI trusted-publisher workflow: `.github/workflows/cd.yml`

## Boundary Contract

`secrets-sync` owns canonical merge, validate, diff, and sync behavior.
`vendor-fabric` should consume the `secrets_sync` binding and provide the
Python facade, provider activation, optional upstream-owned authentication
handoff through `ProviderSession`, redaction, and ExtendedData composition.
`agentic-fabric` should consume vendor capabilities and own runtime/framework
tools. Do not replace the direct gopy binding with a Python CLI wrapper.

## Verification

Run these before handing off or merging:

```bash
go test ./...
GOTOOLCHAIN=go1.25.11 go test ./...
go build -o /tmp/secrets-sync-handoff ./cmd/secrets-sync
go build -o /tmp/secrets-sync-controller-handoff ./cmd/secrets-sync-controller
/tmp/secrets-sync-handoff --help
just lambda-build
python3 -m py_compile tools/check_python_dist.py tools/patch_python_dist.py
just python-matrix
git diff --check
```

`just python-matrix` uses `uv` for Python build dependencies and
`scripts/install-gopy.sh` to install gopy with a current Go-compatible
`golang.org/x/tools` version.

The current binding release path builds per-CPython wheels for Python 3.11
through 3.14. ABI3 would be better long term, but only after the generated C
extension is compiled with `Py_LIMITED_API` and audited; do not merely retag a
version-specific gopy wheel as ABI3.

If `helm` is available, also run:

```bash
helm template secrets-sync ./deploy/charts/secrets-sync >/tmp/secrets-sync-helm-template.yaml
```

Search for old standalone product spellings, old command/chart paths, old
image names, old Kubernetes API group names, and old environment prefixes before
launch.
