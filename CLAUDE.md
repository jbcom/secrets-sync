<!-- profile: standard-repo agent-state v1 -->
# secrets-sync

Enterprise-grade secret synchronization pipeline (two-phase merge → sync) across
cloud providers and secret stores. Go CLI + Kubernetes controller + AWS Lambda,
published as Go module, GitHub Action, Helm chart, and Python gopy wheels.

## Profiles loaded

@/Users/jbogaty/.claude/profiles/agent-state.md
@/Users/jbogaty/.claude/profiles/standard-repo.md

## Repo-specific

Task runner is `just` (Justfile). Go toolchain pinned via `GOTOOLCHAIN=go1.26.4`
(every recipe exports `GO_TOOLCHAIN` fallback). Go is primary; Python bindings
are gopy-generated and built/tested via `tox` (no `pyproject.toml` at root).

- **Build (Go CLI):** `just build` (also `just build-all` → CLI + controller + Lambda)
- **Test unit (Go, race + cover):** `just test-unit`
- **Test Go (plain):** `just test-go`
- **Test integration (LocalStack + Vault):** `just test-integration` (or `just test-integration-docker`)
- **Quality (lint + pytools + docs via tox):** `just quality`
- **Fmt (go fmt):** `just fmt`
- **Tidy (go mod tidy):** `just tidy`
- **Docs (gomarkdoc API + Sphinx, warnings=errors):** `just docs`
- **Clean (bin/dist/.tools/.tox/coverage):** `just clean`
- **Python binding build (one version):** `just python-build [3.13]`
- **Python matrix (3.11–3.14):** `just python-matrix`
- **Vuln scan:** `just vuln`
- **Full local CI surface:** `just ci`

## Notes

- **Root docs-consistency `*_test.go` guards** assert that docs/markets match
  implementation artifacts. Update both sides together:
  `action_docs_test.go` (vs `action.yml`), `dockerfile_test.go` (vs `Dockerfile`),
  `helm_chart_test.go` (vs `deploy/` Helm chart), `workflow_pinning_test.go`
  (vs `.github/workflows/*` pin SHA), `docs_markdown_test.go`,
  `docs_security_test.go`, `docs_version_test.go`, `examples_config_test.go`,
  `kubernetes_crd_test.go`, `release_config_test.go`.
- **In-flight context files** at repo root (not permanent docs):
  `AGENTIC_REINFORCEMENT.md`, `HANDOFF.md`, `SECRETS_SYNC_ALIGNMENT.md`.
  Treat as transient alignment state, not source of truth.
- **Release pipeline order:** ci.yml → release.yml (GoReleaser + Python wheels +
  GHCR) → cd.yml (deploy). release-please-config.json drives versions
  (release-type `go`); do not encode versions in commits or directives.
- **AGENTS.md: missing** — standard-repo requires it for extended operating
  protocols/architecture/patterns. Flag as a gap to create; do not inline that
  content in CLAUDE.md.