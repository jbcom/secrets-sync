# SecretSync Alignment

This file locks the intended SecretSync boundary for the canonical standalone
`jbcom/secrets-sync` repository.

## Canonical Stack

1. `jbcom/secrets-sync` owns the Go runtime, CLI, structured JSON output,
   pipeline semantics, GitHub Action, GHCR distroless image, Helm chart,
   Kubernetes CRD schema and controller, Lambda entrypoint/archive,
   documentation, and gopy binding source.
2. `jbcom/vendor-fabric` consumes that binding from Python and adds
   ExtendedData-aware facade behavior plus credential handoff.
3. `jbcom/agentic-fabric` wraps vendor capabilities for agent runtimes and
   framework-specific tool use.

## Binding Contract

- PyPI distribution: `secrets-sync-python-binding`
- Python import/module: `secrets_sync`
- Expected top-level binding surface includes functions such as
  `ValidateConfig`, `RunPipeline`, `RunPipelineWithSession`, `DryRun`,
  `DryRunWithSession`, `Merge`, `MergeWithSession`, `Sync`,
  `SyncWithSession`, `GetConfigInfo`, `GetTargets`, and `GetSources`, plus
  `NewProviderSession`, `ProviderSession`, `DefaultSyncOptions`,
  `SyncOptions`, `SyncResult`, `ValidationResult`, `StringListResult`, and
  operation/output-format constants.
- `ProviderSession` is the supported direct handoff for upstream-owned Vault
  and AWS authenticated session material. A Python CLI wrapper is not an
  acceptable substitute for the direct gopy binding.
- Local sources that emit legacy closed-up spellings or old Python package
  names should be treated as restoration work, not as the intended public
  contract.

## Required Direction

- Keep merge, validate, diff, and sync semantics canonical here.
- Preserve binding sources here rather than downstream.
- Give `vendor-fabric` a stable contract to wrap instead of pushing pipeline
  ownership into Python.
- Keep the compiled Go artifact reusable across CLI, Action, GHCR image,
  Kubernetes CronJob/controller, Lambda, and Python binding surfaces.

## Forbidden Drift

- Do not declare the binding layer retired while downstream Python still needs
  SecretSync runtime behavior.
- Do not move agent framework wrappers into this repository.
- Do not create a parallel Python business-logic implementation here or ask
  `agentic-fabric` to compensate for vendor/runtime boundary drift.
- Do not tell downstream packages to shell out to the CLI when they need direct
  Python integration; solve the gopy binding.
