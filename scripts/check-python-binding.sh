#!/usr/bin/env bash
set -euo pipefail

python_version="${PYTHON_VERSION:-3.13}"
dist_dir="${PYTHON_DIST_DIR:-python/build/secrets_sync/dist}"
python_tag="cp${python_version/./}"
wheel="$(find "$dist_dir" -name "*${python_tag}"'*.whl' | sort | tail -1)"

if [[ -z "$wheel" ]]; then
  echo "No wheel for Python ${python_version} (${python_tag}) found in $dist_dir" >&2
  exit 1
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

uv venv --python "$python_version" "$tmpdir/.venv" >/dev/null
uv pip install --python "$tmpdir/.venv/bin/python" --force-reinstall "$wheel" >/dev/null

"$tmpdir/.venv/bin/python" - <<'PY'
import secrets_sync

required = [
    "DefaultSyncOptions",
    "NewProviderSession",
    "RunPipeline",
    "RunPipelineWithSession",
    "DryRun",
    "DryRunWithSession",
    "Merge",
    "MergeWithSession",
    "Sync",
    "SyncWithSession",
    "ValidateConfig",
    "GetConfigInfo",
    "GetTargets",
    "GetSources",
    "ProviderSession",
    "ValidationResult",
    "StringListResult",
    "SyncOptions",
    "SyncResult",
]

missing = [name for name in required if not hasattr(secrets_sync, name)]
if missing:
    raise SystemExit(f"missing top-level secrets_sync symbols: {missing}")

opts = secrets_sync.DefaultSyncOptions()
if opts.Operation != secrets_sync.OperationPipeline:
    raise SystemExit(f"default operation mismatch: {opts.Operation!r}")

session = secrets_sync.NewProviderSession()
session.VaultAddress = "https://vault.example.test"
session.VaultNamespace = "platform"
session.VaultToken = "redacted-vault-token"
session.AWSRegion = "us-east-1"
session.AWSAccessKeyID = "AKIAEXAMPLE"
session.AWSSecretAccessKey = "redacted-secret"
session.AWSSessionToken = "redacted-session"
session.AWSEndpointURL = "http://localhost:4566"

if session.VaultAddress != "https://vault.example.test":
    raise SystemExit("provider session VaultAddress was not writable")
if session.AWSRegion != "us-east-1":
    raise SystemExit("provider session AWSRegion was not writable")

validation = secrets_sync.ValidateConfig("/definitely/missing.yaml")
if validation.Valid:
    raise SystemExit("missing config unexpectedly validated")
if not validation.ErrorMessage:
    raise SystemExit("missing config did not return an error message")
PY
