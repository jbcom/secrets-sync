# Agentic Reinforcement

This repository is the standalone `secrets-sync` product surface. It does not
own vendor-provider facades or agent runtime wrappers.

## Required Boundary

- `secrets-sync` owns the canonical Go runtime, CLI, structured JSON output,
  pipeline semantics, GHCR distroless image, GitHub Action, Helm chart,
  Kubernetes CRD schema and controller, Lambda entrypoint/archive,
  documentation, and gopy binding source.
- The Python binding distribution is `secrets-sync-python-binding`.
- The Python import/module is `secrets_sync`.
- `vendor-fabric` consumes the binding and provides an ExtendedData-aware
  facade, provider activation, optional upstream-owned authentication
  handoff through `ProviderSession`, and redaction.
- `agentic-fabric` consumes vendor capabilities and owns framework/runtime
  adapters.

## Do Not Drift

- Do not move agent framework tools into this repository.
- Do not replace the Go runtime with a parallel Python implementation.
- Do not replace the gopy binding with a CLI wrapper for Python.
- Do not revive legacy closed-up Python package names.
- Do not describe the gopy binding as retired while downstream Python still
  depends on this runtime contract.
