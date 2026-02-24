# Python Bindings

SecretSync provides Python bindings via [gopy](https://github.com/go-python/gopy), enabling seamless integration with Python applications, AI agents, and the Extended Data Library ecosystem.

## Overview

The Python bindings expose the core SecretSync functionality:

- **Pipeline execution**: Run merge, sync, or full pipeline operations
- **Configuration validation**: Validate YAML configuration files
- **Dry-run support**: Preview changes without executing them
- **Diff output**: See exactly what would change

## Installation Options

### Option 1: Via vendor-connectors (Recommended)

The easiest way to use SecretSync from Python is via the [vendor-connectors](https://github.com/extended-data-library/vendor-connectors) library:

```bash
pip install vendor-connectors[secrets]
```

This provides:
- Native bindings when available (fastest)
- CLI fallback when bindings aren't installed
- AI framework integrations (LangChain, CrewAI, Strands)
- MCP server support

### Option 2: Build Native Bindings

For maximum performance, build the native Python bindings:

```bash
# Prerequisites
pip install pybindgen build
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/go-python/gopy@latest

# Clone and build
git clone https://github.com/extended-data-library/secretssync.git
cd secretssync
make python-bindings
make python-install
```

### Option 3: CLI-only Mode

If you have the `secretsync` CLI installed, the Python connector will use it automatically:

```bash
go install github.com/extended-data-library/secretssync/cmd/secretsync@latest
pip install vendor-connectors[secrets]
```

## Usage

### Basic Usage

```python
from vendor_connectors.secrets import SecretsConnector

# Initialize connector
connector = SecretsConnector()

# Check which mode is active
print(f"Native bindings: {connector.native_available}")
print(f"CLI available: {connector.cli_available}")

# Validate a configuration file
is_valid, message = connector.validate_config("pipeline.yaml")
if not is_valid:
    print(f"Invalid config: {message}")

# Get configuration info
info = connector.get_config_info("pipeline.yaml")
print(f"Sources: {info.sources}")
print(f"Targets: {info.targets}")
```

### Dry Run

Preview changes without executing:

```python
from vendor_connectors.secrets import SecretsConnector

connector = SecretsConnector()
result = connector.dry_run("pipeline.yaml")

print(f"Would process {result.target_count} targets")
print(f"  Secrets to add: {result.secrets_added}")
print(f"  Secrets to modify: {result.secrets_modified}")
print(f"  Secrets to remove: {result.secrets_removed}")
print(f"  Unchanged: {result.secrets_unchanged}")

# View the diff output
if result.diff_output:
    print("\nDiff:")
    print(result.diff_output)
```

### Running the Pipeline

```python
from vendor_connectors.secrets import (
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
    targets="production,staging",  # Comma-separated string
    parallelism=8,
    continue_on_error=True,
)
result = connector.run_pipeline("pipeline.yaml", options)

if result.success:
    print(f"Synced {result.secrets_added} secrets in {result.duration_ms}ms")
else:
    print(f"Error: {result.error_message}")
```

### Merge and Sync Phases

Run phases independently:

```python
from vendor_connectors.secrets import SecretsConnector

connector = SecretsConnector()

# Run only merge phase (sources → merge store)
merge_result = connector.merge("pipeline.yaml", dry_run=False)

# Run only sync phase (merge store → destinations)
sync_result = connector.sync("pipeline.yaml", dry_run=False)
```

## AI Agent Integration

### LangChain

```python
from vendor_connectors.secrets import get_langchain_tools

tools = get_langchain_tools()

# Use with LangChain agent
from langchain.agents import AgentExecutor
# ... configure agent with tools
```

### CrewAI

```python
from vendor_connectors.secrets import get_crewai_tools
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
from vendor_connectors.secrets import get_strands_tools

tools = get_strands_tools()
# Use as plain functions with Strands agents
```

### Auto-detection

```python
from vendor_connectors.secrets import get_tools

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

The secrets connector is automatically exposed via the vendor-connectors MCP server:

```bash
vendor-connectors-mcp
```

Configure in your MCP client:

```json
{
  "mcpServers": {
    "vendor-connectors": {
      "command": "vendor-connectors-mcp"
    }
  }
}
```

## Performance

| Mode | Relative Speed | Use Case |
|------|----------------|----------|
| Native bindings | 1x (fastest) | Production workloads |
| CLI subprocess | ~2-5x slower | Development, compatibility |

The connector automatically uses native bindings when available.

## Error Handling

```python
from vendor_connectors.secrets import SecretsConnector, SyncResult

connector = SecretsConnector()
result = connector.run_pipeline("pipeline.yaml")

if not result.success:
    print(f"Pipeline failed: {result.error_message}")
    
    # Check detailed results
    import json
    if result.results_json:
        details = json.loads(result.results_json)
        for target_result in details:
            if not target_result.get("success"):
                print(f"  {target_result['target']}: {target_result.get('error')}")
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
