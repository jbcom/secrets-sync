# GitHub Marketplace Listing Guide

This document provides information for listing SecretSync on the GitHub Marketplace as a verified free action.

## Marketplace Information

### Basic Information

**Name**: SecretSync  
**Tagline**: Universal secrets synchronization pipeline for multi-cloud secret management  
**Category**: Deployment and Continuous Integration  
**Pricing**: Free  

### Description

SecretSync provides fully automated, real-time secret synchronization between HashiCorp Vault and AWS. Perfect for multi-account AWS environments, HashiCorp Vault users, and organizations managing secrets across AWS Organizations.

**Key Features:**
- 🔄 Two-phase pipeline architecture (merge → sync)
- 🎯 Vault-to-AWS secrets synchronization
- 🌐 Multi-account AWS secret management
- 📊 GitHub-native diff annotations in PRs
- 🔒 OIDC authentication for AWS (no long-lived credentials)
- 🚀 Dynamic target discovery via AWS Organizations/Identity Center
- ⚡ Zero-configuration Docker action
- 🔐 Complete privacy - no data collection

### Supported Stores

- HashiCorp Vault (KV2) - source
- AWS Secrets Manager - target
- AWS S3 (merge store option)
- Kubernetes Secrets (operator mode)

## Marketplace Requirements Checklist

### ✅ Technical Requirements

- [x] **action.yml file**: Present in repository root
- [x] **Docker-based action**: Uses Dockerfile for containerization
- [x] **Valid inputs**: All inputs properly documented with descriptions
- [x] **Branding**: Icon and color specified
- [x] **Multi-arch support**: Supports linux/amd64 and linux/arm64

### ✅ Documentation Requirements

- [x] **README.md**: Comprehensive documentation with examples
- [x] **Usage examples**: Complete workflow examples
- [x] **Input documentation**: All inputs documented with defaults
- [x] **Quick start guide**: Easy getting started section
- [x] **Advanced examples**: Multiple use case examples

### ✅ Legal and Policy Requirements

- [x] **License**: MIT License (permissive, OSI-approved)
- [x] **Privacy Policy**: See [docs/PRIVACY.md](./PRIVACY.md)
- [x] **Support Information**: See [docs/SUPPORT.md](./SUPPORT.md)
- [x] **Security Policy**: See [docs/SECURITY.md](./SECURITY.md)
- [x] **Code of Conduct**: Implicit in professional conduct

### ✅ Quality Requirements

- [x] **Working action**: Fully functional and tested
- [x] **Error handling**: Proper error messages and exit codes
- [x] **Logging**: Comprehensive logging with multiple formats
- [x] **Security**: No hardcoded secrets, proper secret handling
- [x] **Performance**: Efficient execution with parallel processing

### ✅ Marketplace Best Practices

- [x] **Semantic versioning**: Using git tags (v1, v1.0.0, etc.)
- [x] **Clear naming**: Descriptive and searchable name
- [x] **Useful description**: Clear value proposition
- [x] **Good documentation**: Step-by-step guides and examples
- [x] **Community support**: GitHub Issues and Discussions
- [x] **Regular updates**: Active maintenance and improvements

## Publishing to Marketplace

### Prerequisites

1. **Repository Requirements**
   - Public repository on GitHub
   - Valid `action.yml` in root directory
   - Proper branding (icon, color)
   - Comprehensive README

2. **Legal Requirements**
   - License file (MIT)
   - Privacy policy
   - Support contact information
   - Security policy

3. **Quality Requirements**
   - Working action with examples
   - No security vulnerabilities
   - Proper error handling
   - Good documentation

### Publishing Steps

1. **Verify Action Works**
   ```bash
   # Test action locally
   act -j test-action
   ```

2. **Create Version Tags**
   ```bash
   # Create and push version tags
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   
   # Create major version tag (recommended for users)
   git tag -fa v1 -m "Release v1"
   git push origin v1 --force
   ```

3. **Publish to Marketplace**
   - Go to repository on GitHub
   - Click "Releases"
   - Click "Draft a new release"
   - Choose the version tag (e.g., v1.0.0)
   - Check "Publish this Action to the GitHub Marketplace"
   - Select primary category: "Deployment"
   - Add release notes
   - Click "Publish release"

4. **Verify Listing**
   - Visit: https://github.com/marketplace/actions/secretsync
   - Verify all information is correct
   - Test the action from Marketplace

### Recommended Version Tags

```bash
# Semantic version (specific)
v1.0.0, v1.0.1, v1.1.0, v2.0.0

# Major version (for users - auto-updates)
v1, v2

# Example: Release v1.2.3
git tag -a v1.2.3 -m "Release v1.2.3 - Add OIDC support"
git push origin v1.2.3

# Update major version pointer
git tag -fa v1 -m "Update v1 to v1.2.3"
git push origin v1 --force
```

## Marketplace Metadata

### Action Metadata (action.yml)

```yaml
name: 'SecretSync'
description: 'Universal secrets synchronization pipeline for multi-cloud secret management with Vault, AWS, GCP, and more'
author: 'jbcom'

branding:
  icon: 'lock'
  color: 'blue'
```

### Category Selection

**Primary Category**: Deployment  
**Additional Categories**: 
- Continuous Integration
- Security
- Utilities

### Tags/Keywords

- secrets-management
- vault
- aws-secrets-manager
- secret-sync
- multi-cloud
- devops
- security
- oidc
- ci-cd
- github-actions

## Marketing Copy

### Short Description (200 chars)

Vault-to-AWS secrets sync. Two-phase pipeline with inheritance, dynamic discovery via AWS Organizations/Identity Center & GitHub-native diffs. Free, open source.

### Long Description

SecretSync revolutionizes Vault-to-AWS secrets management with a powerful two-phase pipeline architecture. Built for organizations managing secrets across multiple AWS accounts with HashiCorp Vault as the source of truth.

**Perfect For:**
- Multi-account AWS environments (Control Tower, Organizations)
- HashiCorp Vault users syncing to AWS Secrets Manager
- Teams managing secrets across dev/staging/prod
- Organizations requiring secret inheritance hierarchies
- DevOps teams automating secret distribution

**Key Benefits:**

🔄 **Two-Phase Architecture**
Merge secrets from multiple Vault paths, then sync to multiple AWS accounts with inheritance support.

🎯 **Vault → AWS Sync**
HashiCorp Vault (KV2) as source, AWS Secrets Manager as target, with S3 or Vault merge store options.

🌐 **AWS-Native Patterns**
First-class support for AWS Control Tower, Organizations, and Identity Center patterns.

📊 **GitHub-Native Integration**
Automatic PR annotations, diff reporting, and status checks.

🔒 **Security First**
OIDC authentication, no long-lived credentials, complete audit trail, zero data collection.

🚀 **Dynamic Discovery**
Automatically discover and sync to accounts via AWS Organizations or Identity Center.

**Use Cases:**

1. **Control Tower Environments**: Sync secrets to all AWS accounts in your organization
2. **Vault Distribution**: Push Vault secrets to AWS Secrets Manager across accounts
3. **Secret Inheritance**: Dev → Staging → Production with automatic propagation
4. **Multi-Region**: Sync secrets across AWS regions from central Vault
5. **Compliance**: Automated secret rotation with complete audit trail

**Zero Configuration**
Just add your config file and secrets - the action handles the rest.

## Badge and Shield Links

Add these to README for visibility:

```markdown
[![GitHub Marketplace](https://img.shields.io/badge/Marketplace-SecretSync-blue.svg?colorA=24292e&colorB=0366d6&style=flat&longCache=true&logo=github)](https://github.com/marketplace/actions/secretsync)

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

[![GitHub release](https://img.shields.io/github/release/extended-data-library/secretssync.svg)](https://github.com/extended-data-library/secretssync/releases)

[![GitHub stars](https://img.shields.io/github/stars/extended-data-library/secretssync.svg)](https://github.com/extended-data-library/secretssync/stargazers)
```

## Support URLs

Add these to the Marketplace listing:

- **Documentation**: https://github.com/extended-data-library/secretssync/tree/main/docs
- **Issues**: https://github.com/extended-data-library/secretssync/issues
- **Support**: https://github.com/extended-data-library/secretssync/blob/main/docs/SUPPORT.md
- **Privacy Policy**: https://github.com/extended-data-library/secretssync/blob/main/docs/PRIVACY.md
- **Security**: https://github.com/extended-data-library/secretssync/blob/main/docs/SECURITY.md

## Verification Requirements

For verified publisher status:

1. **Organization Account**: Must be published from an organization (not personal)
2. **Email Verification**: Organization email must be verified
3. **2FA Enabled**: Two-factor authentication required
4. **Quality Standards**: Meet all GitHub Marketplace quality requirements
5. **Active Maintenance**: Regular updates and responsive support

## Monitoring and Maintenance

### Post-Publication Tasks

1. **Monitor Issues**: Respond to bug reports and questions
2. **Track Usage**: Use GitHub's marketplace insights
3. **Regular Updates**: Keep action up-to-date with dependencies
4. **Security Patches**: Respond quickly to security issues
5. **Documentation**: Keep docs updated with new features

### Marketplace Insights

Track these metrics:
- Daily/monthly active users
- Total installations
- Popular use cases (from issues/discussions)
- User feedback and ratings
- Common problems/questions

## Compliance and Policies

### Data Privacy

SecretSync is privacy-by-design:
- ✅ No data collection
- ✅ No external network calls (except to user's configured services)
- ✅ No telemetry or analytics
- ✅ Complete user control
- ✅ Open source and auditable

See [Privacy Policy](./PRIVACY.md) for details.

### Security

- Regular dependency updates
- CodeQL scanning enabled
- Security policy documented
- Responsible disclosure process
- No known vulnerabilities

See [Security Policy](./SECURITY.md) for details.

### Support

- GitHub Issues for bug reports
- GitHub Discussions for Q&A
- Email for security issues
- Community-driven support

See [Support Guide](./SUPPORT.md) for details.

## Frequently Asked Questions

### Can I publish beta/pre-release versions?

Yes! Use pre-release tags:
```bash
git tag -a v1.0.0-beta.1 -m "Beta release"
```

### How do I unpublish from Marketplace?

You can delist the action in your repository settings under "Marketplace".

### Can I charge for this action?

This action is MIT licensed and free. Paid versions require different licensing.

### How do version tags work?

Users can reference:
- `@v1` - Latest v1.x.x (auto-updates)
- `@v1.2.3` - Specific version (pinned)
- `@main` - Latest commit (not recommended)

### What if my action has dependencies?

Docker actions (like this one) bundle all dependencies in the container.

## Additional Resources

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Creating a Docker Container Action](https://docs.github.com/en/actions/creating-actions/creating-a-docker-container-action)
- [Publishing Actions to Marketplace](https://docs.github.com/en/actions/creating-actions/publishing-actions-in-github-marketplace)
- [Marketplace Requirements](https://docs.github.com/en/actions/creating-actions/publishing-actions-in-github-marketplace#requirements-for-publishing-an-action)
- [Action Metadata Syntax](https://docs.github.com/en/actions/creating-actions/metadata-syntax-for-github-actions)

---

**Ready to publish?** Follow the Publishing Steps above and your action will be live on the GitHub Marketplace!
