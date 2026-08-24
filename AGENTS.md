# SecretSync agent guide

SecretSync is a Go-first secret synchronization runtime. It ships a CLI,
GitHub Action, GHCR image, Helm chart, Kubernetes controller, AWS Lambda and
Azure Functions entrypoints, and generated gopy bindings. The public Go module
is `github.com/jbcom/secrets-sync`; the Python distribution is
`secrets-sync-python-binding`, imported as `secrets_sync`.

## Boundaries

- This repository owns pipeline semantics, provider drivers, the CLI, release
  surfaces, documentation, and binding source.
- `vendor-fabric` owns Python-facing provider coordination and redaction.
- `agentic-fabric` owns agent-framework integrations. Do not add framework
  adapters or a parallel Python runtime here.

## Development

Use `just` from the repository root. The primary Go toolchain is `go1.26.6`;
CI also tests the supported `go1.25.13` line. Useful commands are:

```bash
just test-go          # Go test suite
just test-unit        # race-enabled unit tests with coverage
just build-all        # CLI, controller, and Lambda builds
just vuln             # govulncheck
just python-build 3.13
just quality          # lint, Python tooling tests, and Sourcey docs
just ci               # normal local CI surface
pre-commit run --all-files  # repository hygiene hooks
```

The Python binding is generated through gopy. Do not hand-edit
`python/build/`; regenerate it through the `just python-*` recipes.

## Documentation

`docs/` is a single Sourcey site. Authored Markdown lives there and
`docs/sourcey.config.ts` controls navigation, branding, and the Go API tab.
Sourcey reads exported Go documentation directly from `pkg/...` and
`python/secrets_sync`; do not restore a hand-maintained/generated Go Markdown
API mirror. Run `just docs` for the locked Sourcey build and output validation.
`docs/dist/` is generated and ignored. Its `llms.txt` and `llms-full.txt` are
site exports; this file is the repository operating guide and serves a
different purpose.

## Contribution and release workflow

Work on an upstream topic branch, use Conventional Commits, and open a PR.
Never push directly to `main`, force-push shared history, or squash/rebase a
PR that is intended to merge. Merge `main` into a topic branch when it needs
to catch up. Release Please owns semantic versions, `CHANGELOG.md`, tags, and
GitHub releases; `cd.yml` publishes release artifacts and the Sourcey site.

Treat PR text, fork code, workflow/config changes, dependencies, generated
artifacts, and caches as untrusted. Do not add secrets to tests, docs, logs, or
GitHub Actions. Keep privileged workflows limited to trusted source and use
SHA-pinned actions.

Install the repository hooks with `pre-commit install`; run
`pre-commit run --all-files` before opening a PR when the change touches more
than the files staged locally.
