# Python Binding

`secrets-sync` owns the gopy binding source for the Go pipeline runtime.

## Public Names

- PyPI distribution: `secrets-sync-python-binding`
- Python import/module: `secrets_sync`
- Binding source: `python/secrets_sync/secrets_sync.go`
- Generated build output: `python/build/secrets_sync`

## Recommended Python Entry Point

Most Python applications should use the `vendor-fabric` facade:

```bash
pip install "vendor-fabric[secrets-sync]"
```

`vendor_fabric.secrets_sync` should consume the `secrets_sync` binding and add
provider coordination, redaction, data shaping, and ExtendedData-aware
composition. It may either delegate authentication to `secrets-sync` or own the
provider handshake itself and pass the resulting authenticated medium through
`ProviderSession`. It must not reimplement the canonical merge/sync pipeline
semantics in Python.

## Direct Binding Usage

Direct consumers can install the generated binding distribution when they need
the raw Go runtime contract:

```bash
pip install secrets-sync-python-binding
```

```python
import secrets_sync

opts = secrets_sync.DefaultSyncOptions()
opts.DryRun = True
opts.ComputeDiff = True

validation = secrets_sync.ValidateConfig("pipeline.yaml")
if not validation.Valid:
    raise RuntimeError(validation.ErrorMessage)

result = secrets_sync.RunPipeline("pipeline.yaml", opts)
print(result.Success, result.TargetCount, result.DiffOutput)
```

For upstream-owned authentication:

```python
import secrets_sync

session = secrets_sync.NewProviderSession()
session.VaultAddress = "https://vault.example.com"
session.VaultNamespace = "admin"
session.VaultToken = vault_token_from_vendor_fabric
session.AWSRegion = "us-east-1"
session.AWSAccessKeyID = aws_credentials.access_key
session.AWSSecretAccessKey = aws_credentials.secret_key
session.AWSSessionToken = aws_credentials.token

opts = secrets_sync.DefaultSyncOptions()
result = secrets_sync.RunPipelineWithSession("pipeline.yaml", opts, session)
```

## Binding Surface

The intended top-level binding surface includes:

- `ValidateConfig(config_path) -> ValidationResult`
- `RunPipeline(config_path, options) -> SyncResult`
- `RunPipelineWithSession(config_path, options, provider_session) -> SyncResult`
- `DryRun(config_path) -> SyncResult`
- `DryRunWithSession(config_path, provider_session) -> SyncResult`
- `Merge(config_path, dry_run) -> SyncResult`
- `MergeWithSession(config_path, dry_run, provider_session) -> SyncResult`
- `Sync(config_path, dry_run) -> SyncResult`
- `SyncWithSession(config_path, dry_run, provider_session) -> SyncResult`
- `GetConfigInfo(config_path) -> ConfigInfo`
- `GetTargets(config_path) -> StringListResult`
- `GetSources(config_path) -> StringListResult`
- `DefaultSyncOptions() -> SyncOptions`
- `NewProviderSession() -> ProviderSession`
- `ProviderSession`, `SyncOptions`, `SyncResult`, `ConfigInfo`,
  `ValidationResult`, `StringListResult`
- Operation and output-format constants

## Provider Session Contract

`ProviderSession` is a structured handoff of already-authenticated provider
material. It is not a serialized config object and it is not a Python SDK client
pointer. The supported fields are Vault address/namespace/token and AWS
region/static or temporary session credentials, optional role ARN, and optional
endpoint URL. Set `DelegateAuth = True` when the caller wants `secrets-sync` to
use the normal config/environment authentication path instead.

## Local Build

```bash
python -m pip install --upgrade build pybindgen setuptools wheel
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/go-python/gopy@latest
just python-build
```

`just python-build` generates the binding, patches the generated package
metadata to `secrets-sync-python-binding`, builds a wheel, and verifies the
wheel metadata before release.

## Agent Runtime Boundary

Agent framework wrappers do not belong in this repository. `agentic-fabric`
should consume vendor capabilities and own LangChain, CrewAI, LangGraph,
Strands, MCP, or other runtime-specific tool adapters.

## ABI3 Status

ABI3 wheels would be preferable long term because they reduce the release
matrix to one wheel per platform and architecture. Do not retag gopy output as
ABI3 until the generated extension is compiled with CPython's Limited API and
verified with an ABI audit. The launch path is to build and test native wheels
for each supported CPython version: 3.11, 3.12, 3.13, and 3.14.
