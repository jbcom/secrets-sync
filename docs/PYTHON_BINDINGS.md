# Python Integration

SecretSync is available to Python through the `extended-data[secrets]`
connector. That connector executes the supported `secretsync` CLI contract and
returns mapping-style `ExtendedDict` payloads for configuration inspection,
dry-run, merge, sync, and full pipeline operations.

This repository also includes direct [gopy](https://github.com/go-python/gopy)
binding sources under `python/` for local experiments. Those bindings are not
the runtime adapter contract used by `extended-data`.

## Overview

The `extended-data` connector exposes the core SecretSync functionality:

- **Pipeline execution**: Run merge, sync, or full pipeline operations
- **Configuration validation**: Validate YAML configuration files
- **Dry-run support**: Preview changes without executing them
- **Diff output**: See exactly what would change

## Installation Options

### Option 1: Via extended-data (Recommended)

The easiest way to use SecretSync from Python is via the [extended-data](https://github.com/jbcom/extended-data) library:

```bash
pip install extended-data[secrets]
```

This provides:
- CLI execution through the stable `secretsync` command
- Mapping-style `ExtendedDict` results
- AI framework integrations (LangChain, CrewAI, Strands)
- MCP server support

To execute the full pipeline from Python, keep the `secretsync` CLI installed
and available on `PATH`.

### Option 2: Build Direct gopy Bindings

For local experiments with direct Go-to-Python bindings:

```bash
# Prerequisites
pip install pybindgen build
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/go-python/gopy@latest

# Clone and build
git clone https://github.com/jbcom/secrets-sync.git
cd secrets-sync
make python-bindings
make python-install
```

### CLI Requirement

The `extended-data` connector requires the `secretsync` CLI:

```bash
go install github.com/jbcom/secrets-sync/cmd/secretsync@latest
pip install extended-data[secrets]
```

The connector relies on `secretsync pipeline --output json`, which emits the
same stable result envelope for dry-run and apply runs. Diff data is nested
under `diff` and `diff_output` when diff computation is enabled.

## Usage

### Basic Usage

```python
from extended_data.secrets import SecretsConnector

# Initialize connector
connector = SecretsConnector()

# Check that the CLI can be found
print(f"CLI available: {connector.cli_available}")

# Validate a configuration file
validation = connector.validate_config("pipeline.yaml")
if not validation["valid"]:
    print(f"Invalid config: {validation['message']}")

# Get configuration info
info = connector.get_config_info("pipeline.yaml")
print(f"Sources: {info['sources']}")
print(f"Targets: {info['targets']}")
```

### Dry Run

Preview changes without executing:

```python
from extended_data.secrets import SecretsConnector

connector = SecretsConnector()
result = connector.dry_run("pipeline.yaml")

print(f"Would process {result['target_count']} targets")
print(f"  Secrets to add: {result['secrets_added']}")
print(f"  Secrets to modify: {result['secrets_modified']}")
print(f"  Secrets to remove: {result['secrets_removed']}")
print(f"  Unchanged: {result['secrets_unchanged']}")

# Diff output may contain secret values. Inspect it only in a secure terminal
# or through a redacted reporting path.
if result["diff_output"]:
    print("Diff output returned; not printing it from the example.")
```

### Running the Pipeline

```python
from extended_data.secrets import (
    SecretsConnector,
    SyncOptions,
    SyncOperation,
)

connector = SecretsConnector()

# Run with default options (full pipeline)
result = connector.run_pipeline("pipeline.yaml")

# Or customize options
options = SyncOptions(
    operation=SyncOperation.SYNC,  # Only sync phase
    targets=["production", "staging"],
    parallelism=8,
    continue_on_error=True,
)
result = connector.run_pipeline("pipeline.yaml", options)

if result["success"]:
    print(f"Synced {result['secrets_added']} secrets in {result['duration_ms']}ms")
else:
    print("Pipeline failed. Re-run secretsync directly in a secure terminal for diagnostics.")
```

### Merge and Sync Phases

Run phases independently:

```python
from extended_data.secrets import SecretsConnector

connector = SecretsConnector()

# Run only merge phase (sources → merge store)
merge_result = connector.merge("pipeline.yaml", dry_run=False)

# Run only sync phase (merge store → destinations)
sync_result = connector.sync("pipeline.yaml", dry_run=False)
```

## AI Agent Integration

### LangChain

```python
from extended_data.secrets import get_langchain_tools

tools = get_langchain_tools()

# Use with LangChain agent
from langchain.agents import AgentExecutor
# ... configure agent with tools
```

### CrewAI

```python
from extended_data.secrets import get_crewai_tools
from crewai import Agent, Task, Crew

tools = get_crewai_tools()

secrets_agent = Agent(
    role="Secrets Manager",
    goal="Synchronize secrets across environments",
    tools=tools,
)
```

### AWS Strands

```python
from extended_data.secrets import get_strands_tools

tools = get_strands_tools()
# Use as plain functions with Strands agents
```

### Auto-detection

```python
from extended_data.secrets import get_tools

# Automatically detects installed framework
tools = get_tools()
```

## Available Tools

| Tool | Description |
|------|-------------|
| `secrets_validate_config` | Validate a pipeline configuration file |
| `secrets_run_pipeline` | Execute the sync pipeline |
| `secrets_dry_run` | Preview changes without executing |
| `secrets_get_config_info` | Get configuration details |
| `secrets_get_targets` | List sync targets |
| `secrets_get_sources` | List secret sources |

## MCP Server

The secrets connector is automatically exposed via the extended-data MCP server:

```bash
extended-data-mcp
```

Configure in your MCP client:

```json
{
  "mcpServers": {
    "extended-data": {
      "command": "extended-data-mcp"
    }
  }
}
```

## Error Handling

```python
from extended_data.secrets import SecretsConnector

connector = SecretsConnector()
result = connector.run_pipeline("pipeline.yaml")

if not result["success"]:
    print("Pipeline failed. Re-run secretsync directly in a secure terminal for diagnostics.")
    
    # Check detailed results
    import json
    if result["results_json"]:
        details = json.loads(result["results_json"])
        for target_result in details:
            if not target_result.get("success"):
                print(f"  {target_result['target']}: failed")
```

## Environment Variables

The connector respects standard environment variables:

| Variable | Description |
|----------|-------------|
| `VAULT_ADDR` | Vault server address |
| `VAULT_TOKEN` | Vault authentication token |
| `VAULT_ROLE_ID` | AppRole role ID |
| `VAULT_SECRET_ID` | AppRole secret ID |
| `AWS_REGION` | AWS region |
| `AWS_ACCESS_KEY_ID` | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key |
