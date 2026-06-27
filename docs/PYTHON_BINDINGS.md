# Python Integration

Python-native SecretSync capabilities live in
[`jbcom/vendor-fabric`](https://github.com/jbcom/vendor-fabric), not this
repository.

Install the Python surface with:

```bash
pip install "vendor-fabric[secrets-sync]"
```

Use `vendor_fabric.secrets_sync` for Python applications that need to validate
pipeline configuration, run dry-runs, merge sources, sync targets, or compose
SecretSync with vendor connectors and Extended Data primitives.

This repository keeps the standalone Go CLI, GitHub Action, Helm chart, and
Go release flow. The retired Python package and generated binding source were
removed so Python users get one first-class implementation instead of a bridge
over a separate runtime.

For shell, CI, and scheduled execution, continue to use the Go CLI contract:

```bash
secrets-sync validate --config pipeline.yaml
secrets-sync pipeline --config pipeline.yaml --dry-run --diff --output json
secrets-sync pipeline --config pipeline.yaml --output json
```
