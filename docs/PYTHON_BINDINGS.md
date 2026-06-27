# Python Integration

SecretSync is available to Python through the `secrets-sync-bridge` package.
The bridge uses one stable Python API over two runtimes:

- `backend="auto"` uses local gopy bindings when `secrets_sync_native` is
  installed, then falls back to the `secrets-sync` CLI.
- `backend="cli"` requires the `secrets-sync` CLI.
- `backend="native"` requires the generated `secrets_sync_native` module.

All modes return redacted `ExtendedDict` payloads for configuration inspection,
dry-run, merge, sync, and full pipeline operations.

The bridge depends on `extended-data` for structured data boundaries,
redaction, and export helpers. It does not inherit from Extended Data classes,
and it accepts either a standard `logging.Logger`-style object or an
`extended_data.Logging` lifecycle logger.

This repository also includes direct [gopy](https://github.com/go-python/gopy)
binding sources under `python/secrets_sync`. Generated bindings intentionally
use the separate import package `secrets_sync_native` so they do not collide
with the published bridge package import, `secrets_sync`.

## Installation

```bash
pip install secrets-sync-bridge
```

For CLI mode, install the CLI and keep it available on `PATH`:

```bash
go install github.com/jbcom/secrets-sync/cmd/secrets-sync@latest
```

The stable CLI contract is `secrets-sync pipeline --output json`.

## Basic Usage

```python
from secrets_sync import SecretsSyncBridge

bridge = SecretsSyncBridge()

validation = bridge.validate_config("pipeline.yaml")
if not validation["valid"]:
    raise SystemExit(validation["message"])

info = bridge.get_config_info("pipeline.yaml")
result = bridge.dry_run("pipeline.yaml")

assert "sources" in info
assert "targets" in info
assert result["secrets_processed"] >= 0
```

## Pipeline Options

```python
from secrets_sync import SecretsSyncBridge, SyncOperation, SyncOptions

bridge = SecretsSyncBridge()

result = bridge.run_pipeline(
    "pipeline.yaml",
    SyncOptions(
        operation=SyncOperation.SYNC,
        targets=["production", "staging"],
        parallelism=8,
        continue_on_error=True,
    ),
)

if not result["success"]:
    raise SystemExit("Pipeline failed; run secrets-sync directly in a secure terminal for diagnostics.")
```

## Merge And Sync Phases

```python
from secrets_sync import SecretsSyncBridge

bridge = SecretsSyncBridge()

merge_result = bridge.merge("pipeline.yaml", dry_run=True)
sync_result = bridge.sync("pipeline.yaml", dry_run=True)

assert "success" in merge_result
assert "success" in sync_result
```

## Logger Integration

```python
from extended_data import Logging
from secrets_sync import SecretsSyncBridge

logger = Logging(logger_name="secrets-sync", enable_console=False, enable_file=False)
bridge = SecretsSyncBridge(logger=logger)

assert bridge.cli_available in {True, False}
assert bridge.native_available in {True, False}
```

## Agent Tools

Agent framework integrations live in `jbcom/agent-orchestration`, where optional framework
dependencies already belong. Use `agentic-crew[secrets-sync]` when a CrewAI,
LangChain, LangGraph, or Strands workflow needs SecretSync tools. This keeps
`secrets-sync-bridge` as a narrow Python interface over the Go CLI contract.

## Direct gopy Runtime

For local direct Go-to-Python bindings:

```bash
pip install pybindgen build
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/go-python/gopy@latest
make python-bindings
make python-install
```

The `make python-bindings` target generates a `secrets_sync_native` package.
After installing it, `SecretsSyncBridge(backend="auto")` will use native mode;
`SecretsSyncBridge(backend="native")` requires it and reports a clear error
when it is missing.
