# secrets-sync-bridge

Python bridge for the `secrets-sync` CLI and local gopy bindings.

```bash
pip install secrets-sync-bridge
```

```python
from secrets_sync import SecretsSyncBridge

bridge = SecretsSyncBridge()
result = bridge.dry_run("pipeline.yaml")
```

`SecretsSyncBridge()` defaults to `backend="auto"`, which uses an installed
`secrets_sync_native` gopy module when present and otherwise falls back to the
`secrets-sync` CLI. Use `backend="cli"` or `backend="native"` to require one
runtime explicitly.
