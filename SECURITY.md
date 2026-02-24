# Security Policy

## Supported Versions

We actively support the following versions of SecretSync with security updates:

| Version | Supported          |
| ------- | ------------------ |
| 1.2.x   | ✅ Yes             |
| 1.1.x   | ✅ Yes             |
| 1.0.x   | ⚠️ Critical fixes only |
| < 1.0   | ❌ No              |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report security vulnerabilities through one of the following methods:

### GitHub Security Advisories (Preferred)

1. Go to the [Security Advisories](https://github.com/extended-data-library/secretssync/security/advisories) page
2. Click "Report a vulnerability"
3. Fill out the form with details about the vulnerability

### Email (Alternative)

If you cannot use GitHub Security Advisories, please open a confidential discussion at [GitHub Discussions](https://github.com/jbcom/go-secretsync/discussions) with the label "security".

Include the following information:
- Type of issue (e.g. buffer overflow, SQL injection, cross-site scripting, etc.)
- Full paths of source file(s) related to the manifestation of the issue
- The location of the affected source code (tag/branch/commit or direct URL)
- Any special configuration required to reproduce the issue
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the issue, including how an attacker might exploit the issue

## Response Timeline

- **Initial Response**: Within 48 hours of report
- **Confirmation**: Within 7 days of report
- **Fix Development**: Varies based on complexity
- **Public Disclosure**: After fix is available and deployed

## Security Best Practices

When using SecretSync, please follow these security best practices:

### Configuration Security

- **Never commit secrets**: Use environment variables for all sensitive data
- **Sanitize configurations**: Remove credentials before sharing configs
- **Use least privilege**: Grant minimal necessary permissions to IAM roles
- **Rotate credentials**: Regularly rotate Vault and AWS credentials

### Deployment Security

- **Use OIDC**: Prefer OIDC over long-lived credentials in CI/CD
- **Network isolation**: Deploy in private networks when possible
- **TLS everywhere**: Ensure all connections use TLS
- **Regular updates**: Keep SecretSync updated to latest version

### Operational Security

- **Monitor logs**: Watch for unusual activity in logs
- **Audit access**: Regularly review who has access to secrets
- **Backup secrets**: Maintain secure backups of critical secrets
- **Test recovery**: Regularly test secret recovery procedures

## Known Security Considerations

### Secrets in Memory

SecretSync temporarily holds secrets in memory during processing. While we clear sensitive data after use, memory dumps could potentially expose secrets.

**Mitigation**: Run SecretSync in secure environments with memory protection.

### Log Security

SecretSync never logs secret values, but it does log paths and metadata. Ensure log aggregation systems are properly secured.

**Mitigation**: Use structured logging and secure log storage.

### Network Security

SecretSync makes network calls to Vault and AWS. Ensure these connections are properly secured.

**Mitigation**: Use TLS, VPNs, or private networks for all connections.

## Security Features

SecretSync includes several security features:

- **Value Masking**: Sensitive values are masked in diff output by default
- **Path Validation**: Prevents path traversal attacks
- **Circuit Breakers**: Prevents cascade failures that could expose systems
- **Request Tracking**: Enables audit trails for debugging
- **No Disk Storage**: Secrets are never written to disk unencrypted

## Vulnerability Disclosure Policy

We follow responsible disclosure practices:

1. **Private Reporting**: Vulnerabilities are reported privately first
2. **Coordinated Disclosure**: We work with reporters to understand and fix issues
3. **Public Disclosure**: Details are made public after fixes are available
4. **Credit**: Security researchers are credited for their findings (if desired)

## Security Updates

Security updates are released as:

- **Patch releases** for supported versions (e.g., 1.2.1 → 1.2.2)
- **Security advisories** published on GitHub
- **Release notes** highlighting security fixes

Subscribe to releases to stay informed about security updates.

## Bug Bounty

We do not currently offer a formal bug bounty program, but we greatly appreciate security research and will acknowledge contributors in our security advisories.

## Contact

For security-related questions or concerns:

- **Security Issues**: Use GitHub Security Advisories
- **General Security Questions**: Create a GitHub Discussion
- **Documentation Issues**: Create a regular GitHub Issue

Thank you for helping keep SecretSync and our users safe!