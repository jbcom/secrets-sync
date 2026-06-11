# Security

As this service is effectively an event bus for your various secret stores and therefore must be granted read and write access to those stores, understanding your security posture is critical. Throughout the documentation presented in this respository, every effort has been made to present both the "less-secure" and "production-grade" approaches, and to call out where security decisions must be made. With that said, as with any solution you adopt, it is critical to understand the security implications of the decisions you make. The solution is released as MIT licensed open source software, and as such, the authors and contributors cannot be held responsible for any security incidents that may occur as a result of using this software.

## Design Considerations

### Secure by Default

The service is designed to be secure by default, preferrring more explicit configuration as opposed to insecure defaults. This does mean initial set up will not be one-liner fire-and-forget. That is intentional, as it forces you to think about the security implications of the decisions you are making rather than starting up the process insecurely, forgetting about it, and then being surprised when something goes wrong. If started with no configuration and no command line flags, the service will exit with an error. All components of the service default to disabled, you must explicitly enable the components you wish to use.

### Logging and Diagnostics

SecretSync logs operational metadata for troubleshooting, not secret material.
Vault and AWS client initialization logs are limited to explicit non-secret
fields such as address, path, auth method, region, role ARN, cache settings, and
boolean capability flags. Raw secret values, raw Vault secret response objects,
raw AWS secret response objects, and raw client structures must not be written
to logs at any level.

Diagnostics may include secret paths, target names, account IDs, error classes,
request IDs, durations, counts, and operation names. Treat those logs as
operationally sensitive, but they should not contain the bytes being synced.
Keep `--log-level debug` and `--log-level trace` restricted to trusted
operators and secured log sinks.

Machine-readable `secretsync pipeline --output json` result envelopes redact
common secret-bearing diagnostic fragments in top-level and per-target error
strings before serialization, including bearer tokens, password or token
assignments, API key assignments, client secrets, and matching URL query
parameters. Downstream consumers should still treat `error_message`, per-target
`error`, and `diff_output` as operationally sensitive and avoid copying them to
untrusted logs, comments, or chat systems without their own policy checks.
GitHub Actions annotation output escapes workflow-command data in target names
and secret paths before writing groups, notices, or warnings.

### Segregation of Duties

SecretSync runs as an explicit pipeline command. Split duties by separating
configuration authors, CI/CD approvers, runtime identities, and target account
roles. In Kubernetes, prefer a scheduled job with a dedicated service account
and narrowly scoped projected credentials.

## Security Configuration

### Pipeline Runner

The supported runtime surface is `secretsync pipeline`. It does not expose an
ingress API by default. Network access should be outbound-only to the configured
secret stores and cloud provider APIs unless your environment adds its own
wrapping service.

### Secret Stores

The service must be granted read and write access to the secret stores it is syncing to. This is a critical security consideration, as the service will be able to read and write secrets to the destination store. In a sync operation, the service will only read from the source and write to the destination.

It is recommended to use a service account with the least privileges necessary to perform the sync operation. For example, if you are syncing to a Vault instance, you should create a policy that only allows the service to read and write to the paths it needs to sync.

All operations performed by the service include an `X-Vault-Sync: true` header to identify the action as being performed by the sync service.

The pipeline runtime is highly privileged relative to the stores it can read
from and write to. Keep configuration changes behind review, protect the merge
store, and scope runtime identities to only the source paths and target accounts
the pipeline needs.

Every sync operation will instantiate a new client to the source and destination secret store, and will close the client after the operation is complete. At no time are client objects reused between sync operations. This is to ensure that the client is not left open and vulnerable to attack.

#### AWS

If you are running in AWS EKS, use workload identity or IRSA for the pipeline
runner. If you are running in a different environment, you can use
`AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`, but prefer short-lived
credentials and rotate any static keys regularly.

For cross-account access you must configure an IAM role in your target account
which can be assumed by the identity associated with the pipeline runner.

Your role will need to have the following permissions:

- `secretsmanager:CreateSecret`
- `secretsmanager:UpdateSecret`
- `secretsmanager:PutSecretValue`
- `secretsmanager:DeleteSecret`
- `secretsmanager:ReplicateSecretToRegions`
- `secretsmanager:RemoveRegionsFromReplication`
- `secretsmanager:ListSecrets`
- `secretsmanager:ListSecretVersionIds`
- `secretsmanager:DescribeSecret`
- `secretsmanager:TagResource`

Here is an example policy document:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "secretsmanager:CreateSecret",
                "secretsmanager:UpdateSecret",
                "secretsmanager:PutSecretValue",
                "secretsmanager:DeleteSecret",
                "secretsmanager:ReplicateSecretToRegions",
                "secretsmanager:RemoveRegionsFromReplication",
                "secretsmanager:ListSecrets",
                "secretsmanager:ListSecretVersionIds",
                "secretsmanager:DescribeSecret",
                "secretsmanager:TagResource"
            ],
            "Resource": "*"
        }
    ]
}
```

and an example trusted entity configuration:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::1234567890:role/secretsync"
      },
      "Action": ["sts:AssumeRole", "sts:TagSession"],
    }
  ]
}
```

### HashiCorp Vault

If you are running in Kubernetes, you can use the Kubernetes auth method to
authenticate the pipeline runner with Vault. If you are running in a different
environment, use AppRole or `VAULT_TOKEN`. Rotate long-lived credentials
regularly and prefer a secret manager or workload identity mechanism to inject
them into the runtime environment.


## Vulnerability Reporting

If you believe you have found a security vulnerability in this project, please
report it privately through
[GitHub Security Advisories](https://github.com/jbcom/secrets-sync/security/advisories)
or email [security@jbcom.dev](mailto:security@jbcom.dev). If you are unsure
whether the issue is a security vulnerability, please report it anyway. We take
all reports seriously and will respond promptly to your inquiry. Please do not
disclose the issue publicly until we have had a chance to address it.
