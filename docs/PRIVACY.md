# Privacy Policy for SecretSync

**Effective Date:** December 8, 2024  
**Last Updated:** December 8, 2024

## Introduction

SecretSync is a free, open-source GitHub Action for secrets synchronization. This privacy policy describes what data SecretSync collects and how it's used.

## Data Collection

### Data We DO NOT Collect

SecretSync **does not collect, store, or transmit** any user data to external servers. Specifically:

- ❌ No secret values or credentials are collected
- ❌ No configuration data is sent to third parties
- ❌ No telemetry or analytics are collected
- ❌ No usage statistics are tracked
- ❌ No personal information is gathered
- ❌ No logs are sent to external services

### How SecretSync Works

SecretSync runs entirely within your GitHub Actions workflow environment:

1. **Execution**: Runs in a Docker container within GitHub's infrastructure
2. **Configuration**: Reads configuration from your repository files
3. **Authentication**: Uses credentials you provide via GitHub Secrets or OIDC
4. **Operations**: Connects directly to your Vault, AWS, GCP, or other services
5. **Logging**: All logs remain within GitHub Actions log system

### Data Flow

```
┌─────────────────────────────────────────────────────────────┐
│  GitHub Actions Runner (Your Controlled Environment)        │
│                                                              │
│  ┌──────────────┐                                           │
│  │  SecretSync  │ ──────┐                                   │
│  │   Container  │       │                                   │
│  └──────────────┘       │                                   │
│         │               │                                   │
│         ├───────────────┴─────► Your Vault Instance         │
│         │                                                    │
│         ├──────────────────────► Your AWS Accounts          │
│         │                                                    │
│         └──────────────────────► Your GCP Projects          │
│                                                              │
│  No data leaves your controlled infrastructure              │
└─────────────────────────────────────────────────────────────┘
```

## Third-Party Services

SecretSync connects to services **you configure**, which may include:

- **HashiCorp Vault**: Your own Vault instance
- **AWS Services**: Your AWS accounts (Secrets Manager, S3, Organizations, etc.)
- **GCP Services**: Your GCP projects (Secret Manager)
- **Other Secret Stores**: Any services you configure in your config file

Each of these services has its own privacy policy and data handling practices. SecretSync acts as a client to these services on your behalf.

## GitHub Marketplace

When you use SecretSync through GitHub Marketplace:

- **Installation**: GitHub may collect installation data per their [GitHub Privacy Statement](https://docs.github.com/en/site-policy/privacy-policies/github-privacy-statement)
- **Usage**: GitHub Actions logs may contain information you choose to log
- **No Additional Data**: SecretSync does not add any additional data collection beyond what GitHub already provides

## Data Security

### In Transit

- All connections use TLS/HTTPS encryption
- Credentials are passed via environment variables (GitHub Secrets)
- No credentials are logged or exposed

### At Rest

- SecretSync does not persist any data
- Container is ephemeral and destroyed after each run
- No local storage or caching of secrets

## Your Rights

Since SecretSync doesn't collect any personal data:

- **Access**: Not applicable - no data is collected
- **Deletion**: Not applicable - no data is stored
- **Portability**: Not applicable - no data is collected
- **Correction**: Not applicable - no data is stored

## Audit and Compliance

### Open Source

SecretSync is fully open source:
- **Source Code**: Available at [github.com/extended-data-library/secretssync](https://github.com/extended-data-library/secretssync)
- **Transparency**: All code is reviewable
- **Community**: Issues and improvements are publicly tracked

### Audit Trail

All SecretSync operations are logged in your GitHub Actions logs, which you control:
- View logs in the Actions tab of your repository
- Configure log retention per your organization's policy
- Export logs for compliance requirements

## Changes to This Policy

We may update this privacy policy from time to time. Changes will be:
- Posted to the [SecretSync repository](https://github.com/extended-data-library/secretssync)
- Documented in the CHANGELOG
- Effective immediately upon posting

## Contact

For privacy-related questions or concerns:

- **Email**: [Contact via GitHub](https://github.com/jbcom)
- **Issues**: [GitHub Issues](https://github.com/extended-data-library/secretssync/issues)
- **Security**: See [SECURITY.md](https://github.com/extended-data-library/secretssync/blob/main/docs/SECURITY.md)

## Compliance

### GDPR Compliance

SecretSync is GDPR-compliant by design:
- No personal data collection
- No data processing
- No data storage
- Complete user control

### SOC 2 / ISO 27001

Organizations using SecretSync for compliance:
- Maintain complete control over all data
- Can audit all source code
- Own all logs and execution environments
- Control all access credentials

## License

SecretSync is licensed under the [MIT License](https://github.com/extended-data-library/secretssync/blob/main/LICENSE).

---

**Summary**: SecretSync collects zero data. It's a tool that runs in your environment, using your credentials, to manage your secrets. All data remains under your control.
