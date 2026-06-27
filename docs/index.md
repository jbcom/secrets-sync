# secrets-sync

secrets-sync is a Go CLI, GitHub Action, GHCR distroless image, Helm chart,
Kubernetes `CredentialSynchronization` controller, AWS Lambda entrypoint, and
gopy binding for synchronizing secrets through a two-phase merge and sync
pipeline. Python applications can import the repo-owned `secrets_sync` binding
from the `secrets-sync-python-binding` distribution. Downstream facades such as
vendor-fabric may wrap that binding, but they are not shipped by this repository.

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
