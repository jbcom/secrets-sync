# SecretSync

SecretSync is a Go CLI, GitHub Action, GHCR distroless image, Helm chart,
Kubernetes `CredentialSynchronization` controller, AWS Lambda entrypoint, and
gopy binding for synchronizing secrets through a two-phase merge and sync
pipeline. Python applications should normally use the
`vendor_fabric.secrets_sync` facade, which wraps this repository's
`secrets_sync` binding with vendor and Extended Data coordination.

```{toctree}
:caption: Guides
:maxdepth: 2

GETTING_STARTED
getting-started/installation
getting-started/quickstart
USAGE
PIPELINE
ARCHITECTURE
ARCHITECTURE_AUDIT
TWO_PHASE_ARCHITECTURE
DEPLOYMENT
GITHUB_ACTIONS
ACTION_QUICK_REFERENCE
OBSERVABILITY
ERROR_CONTEXT
OWNERSHIP
api/index
SECURITY
PRIVACY
SUPPORT
FAQ
MARKETPLACE
ROADMAP
PUBLISHING_CHECKLIST
```

```{toctree}
:caption: Development
:maxdepth: 2

development/contributing
testing/organizations-discovery-integration-tests
```

```{toctree}
:caption: Python
:maxdepth: 2

PYTHON_BINDINGS
```
