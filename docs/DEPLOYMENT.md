# Deployment

SecretSync deploys as a `secrets-sync pipeline` runner. The current production
surface is the CLI, the GHCR Docker image, the GitHub Action, the Lambda
runtime, or a Kubernetes workload that invokes the same pipeline command with a
mounted configuration file.

## Deployment Contract

- Use one pipeline configuration file as the source of truth.
- Validate the configuration before the first apply run.
- Run `--dry-run --diff` before writes in CI and scheduled jobs.
- Enable machine-readable output for automation with `--output json` or
  `--output github`.
- Use `--exit-code` when a CI job must distinguish no changes, changes, and
  failures.
- Enable metrics with `--metrics-port` for long-running or scheduled runners.

## Configuration

Create a pipeline configuration with Vault source settings, a merge store, AWS
execution context, and targets. See [PIPELINE.md](./PIPELINE.md) for the full
schema.

```yaml
vault:
  address: https://vault.example.com/
  namespace: eng/data-platform
  auth:
    approle:
      role_id: ${VAULT_ROLE_ID}
      secret_id: ${VAULT_SECRET_ID}

aws:
  region: us-east-1
  execution_context:
    type: delegated_admin
    account_id: "123456789012"
  control_tower:
    enabled: true
    execution_role:
      name: AWSControlTowerExecution

sources:
  analytics:
    vault:
      mount: analytics

merge_store:
  vault:
    mount: merged-secrets

targets:
  Serverless_Stg:
    account_id: "111111111111"
    imports:
      - analytics
```

Validate before deploying:

```bash
secrets-sync validate --config config.yaml
secrets-sync graph --config config.yaml
```

## Local Or VM Runner

Install the binary and run the same command used in CI:

```bash
go install github.com/jbcom/secrets-sync/cmd/secrets-sync@latest

secrets-sync pipeline \
  --config config.yaml \
  --dry-run \
  --diff \
  --output json \
  --exit-code
```

For an apply run, remove `--dry-run` after reviewing the diff:

```bash
secrets-sync pipeline --config config.yaml --diff --output json
```

## Docker Runner

Mount the configuration read-only and pass credentials through environment
variables, workload identity, or mounted secret files.

```bash
docker run --rm \
  -v "$PWD/config.yaml:/config.yaml:ro" \
  -e VAULT_ADDR=https://vault.example.com \
  -e VAULT_ROLE_ID="$VAULT_ROLE_ID" \
  -e VAULT_SECRET_ID="$VAULT_SECRET_ID" \
  ghcr.io/jbcom/secrets-sync:v2.3.1 \
  pipeline --config /config.yaml --dry-run --diff --output json
```

Use the same image for scheduled container platforms. The image is a Google
Distroless static runtime with no shell or package manager. It contains both
`secrets-sync` and `secrets-sync-controller`; the default entry point is
`secrets-sync`, and the default command is `pipeline`.

## GitHub Actions

Use the published Docker action for CI and release pipelines. Keep the action
reference on a release tag and configure AWS/Vault credentials before invoking
SecretSync.

```yaml
jobs:
  secrets:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      id-token: write
    steps:
      - uses: actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6.0.3
      - uses: aws-actions/configure-aws-credentials@e7f100cf4c008499ea8adda475de1042d6975c7b # v6.2.0
        with:
          role-to-assume: arn:aws:iam::123456789012:role/SecretSyncRunner
          aws-region: us-east-1
      - uses: jbcom/secrets-sync@vX.Y.Z
        with:
          config: config.yaml
          dry-run: "true"
          compute-diff: "true"
          output-format: json
          exit-code: "true"
```

See [GITHUB_ACTIONS.md](./GITHUB_ACTIONS.md) for the full action input
reference.

## Kubernetes CronJob

For Kubernetes, run SecretSync as a scheduled job when one fixed pipeline
configuration is enough. Mount the pipeline configuration from a ConfigMap or
Secret and provide credentials through your cluster identity model.

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: secrets-sync
spec:
  schedule: "*/30 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: Never
          serviceAccountName: secrets-sync
          containers:
            - name: secrets-sync
              image: ghcr.io/jbcom/secrets-sync:v2.3.1
              args:
                - pipeline
                - --config
                - /config/config.yaml
                - --diff
                - --output
                - json
              volumeMounts:
                - name: config
                  mountPath: /config
                  readOnly: true
          volumes:
            - name: config
              configMap:
                name: secrets-sync-config
```

For Kubernetes-authenticated Vault access, configure the `vault.auth.kubernetes`
section in the pipeline file and bind the service account to the expected Vault
role. For AWS, prefer IRSA, EKS Pod Identity, or another workload identity
mechanism over static access keys.

### Kubernetes Controller

`deploy/crds/secrets-sync.jbcom.dev_credentialsynchronizations.yaml` defines the
`secrets-sync.jbcom.dev/v1alpha1` `CredentialSynchronization` API for
declarative scheduled synchronization. The controller in
`cmd/secrets-sync-controller` watches those resources and reconciles each one
into a managed `CronJob` that invokes `secrets-sync pipeline`.

```bash
kubectl apply -f deploy/crds/secrets-sync.jbcom.dev_credentialsynchronizations.yaml
kubectl apply -k deploy/controller
kubectl apply -f deploy/crds/examples/kubernetes-credential-synchronization.yaml
```

The direct manifests install a `secrets-sync` namespace, controller service
account, RBAC, and Deployment using `ghcr.io/jbcom/secrets-sync:v2.3.1`. Keep
that value pinned to an immutable release tag or digest in production.

The Helm chart supports both deployment paths. Enable `pipeline.enabled` for a
single direct CronJob, or enable `controller.enabled` to install the controller
Deployment and RBAC:

```bash
helm upgrade --install secrets-sync deploy/charts/secrets-sync \
  --namespace secrets-sync \
  --create-namespace \
  --set controller.enabled=true
```

## AWS Lambda

Release assets include Lambda archives built from the Go entrypoint at
`cmd/secrets-sync-lambda`. The Lambda handler accepts one of:

- `config_yaml` for inline configuration.
- `config_s3_bucket` and `config_s3_key` for S3-hosted configuration.
- `config_path` or `SECRETS_SYNC_CONFIG` for packaged configuration.

Build locally with:

```bash
just lambda-build
```

Deploy the SAM/CloudFormation template in `deploy/lambda/template.yaml` after
uploading the release archive or local `dist/lambda/bootstrap` package to your
deployment bucket. The Lambda response is structured JSON with the same counts,
target status, duration, and optional diff output exposed by the CLI/binding
surfaces.

## Metrics And Logs

Enable metrics when the runner stays alive long enough to be scraped, or when a
platform sidecar captures process metrics:

```bash
secrets-sync pipeline --config config.yaml --metrics-addr 0.0.0.0 --metrics-port 9090
```

Use JSON logs in centralized logging environments:

```bash
secrets-sync pipeline --config config.yaml --log-format json --log-level info
```

SecretSync logs operational metadata, paths, targets, counts, durations, and
provider error context. It must not log raw secret values, raw Vault secret
payloads, raw AWS secret payloads, or raw client structures.

## Rollout Checklist

1. Validate the configuration with `secrets-sync validate`.
2. Render and review the dependency graph with `secrets-sync graph`.
3. Run a dry-run diff with machine-readable output.
4. Confirm AWS role assumption and Vault authentication from the runner.
5. Run the apply command with `--diff` enabled for auditability.
6. Monitor exit status, logs, and metrics after each scheduled run.
