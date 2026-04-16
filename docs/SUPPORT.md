# Support and Contact

## Getting Help

We're here to help you get the most out of SecretSync! Here are the best ways to get support:

## 📚 Documentation

Start with our comprehensive documentation:

- **[README](../README.md)**: Overview and quick start
- **[GitHub Actions Guide](./GITHUB_ACTIONS.md)**: Complete GitHub Actions usage
- **[Pipeline Configuration](./PIPELINE.md)**: Configuration reference
- **[Security Guide](./SECURITY.md)**: Security best practices
- **[Two-Phase Architecture](./TWO_PHASE_ARCHITECTURE.md)**: Architecture details
- **[Examples](../examples/)**: Working examples and templates

## 💬 Community Support

### GitHub Issues

Use repo issues for bug reports, feature ideas, and how-to questions:

**[GitHub Issues](https://github.com/jbcom/extended-data-library/issues)**

Before opening an issue:
1. Search existing issues to avoid duplicates
2. Use issue templates when available
3. Include all requested information

**Bug Report Template:**
```markdown
## Description
Brief description of the bug

## Steps to Reproduce
1. Step one
2. Step two
3. Step three

## Expected Behavior
What should happen

## Actual Behavior
What actually happens

## Environment
- SecretSync version:
- GitHub Actions runner:
- Configuration: (sanitized YAML excerpt)

## Logs
```
(Paste relevant logs here - sanitize any secrets!)
```
```

## 🔒 Security Issues

**IMPORTANT**: For security vulnerabilities, please report privately.

### How to Report Security Issues

1. **GitHub Security Advisories** (Recommended)
   - Go to: https://github.com/jbcom/extended-data-library/security/advisories
   - Click "Report a vulnerability"
   - Provide details privately

2. **Email** (Alternative)
   - Contact: security@jbcom.dev

3. **Response Time**
   - We aim to respond within 48 hours
   - Critical issues are prioritized

**DO NOT** open public issues for security vulnerabilities.

## 📧 Direct Contact

For other inquiries:

- **Maintainer**: [@jbcom](https://github.com/jbcom)
- **Organization**: [jbcom on GitHub](https://github.com/jbcom)

## 🐛 Reporting Bugs

When reporting bugs, please include:

1. **Version Information**
   ```bash
   # If using CLI
   secretsync --version
   
   # If using GitHub Action
   # Include the version/tag from your workflow
   uses: jbcom/extended-data-library/packages/secretssync@secretssync-v2.0.2
   ```

2. **Configuration** (sanitized - remove secrets!)
   ```yaml
   # Include relevant parts of your config.yaml
   # Replace sensitive values with placeholders
   ```

3. **Logs** (sanitized!)
   ```
   # GitHub Actions logs or CLI output
   # REMOVE any secret values before sharing
   ```

4. **Expected vs Actual Behavior**
   - What you expected to happen
   - What actually happened

## 🎯 Feature Requests

We welcome feature requests! When requesting a feature:

1. **Check Existing Requests**: Search existing issues first
2. **Describe the Use Case**: Why is this feature needed?
3. **Propose a Solution**: How should it work?
4. **Consider Alternatives**: What workarounds exist today?

Use this template:

```markdown
## Feature Request

### Use Case
Why is this feature needed? What problem does it solve?

### Proposed Solution
How should this feature work?

### Alternatives Considered
What other approaches could solve this problem?

### Additional Context
Any other relevant information
```

## 🤝 Contributing

We welcome contributions! See our contributing guidelines:

1. **Fork the Repository**
2. **Create a Feature Branch**
   ```bash
   git checkout -b feature/amazing-feature
   ```
3. **Make Your Changes**
   - Follow existing code style
   - Add tests for new features
   - Update documentation
4. **Test Your Changes**
   ```bash
   go test ./...
   golangci-lint run
   ```
5. **Submit a Pull Request**

### Code of Conduct

Be respectful and inclusive. We're all here to learn and improve.

## 📖 Learning Resources

### Example Workflows

See our [examples directory](../examples/) for:
- Basic GitHub Actions workflow
- Multi-environment setup
- Dynamic discovery patterns
- OIDC authentication examples

### Video Tutorials

*(Coming soon - contributions welcome!)*

### Blog Posts

*(Coming soon - share yours!)*

## 🏢 Enterprise Support

For enterprise needs:

- **Custom Integration**: Contact us for custom integration support
- **Training**: Available for team training and onboarding
- **SLA**: Dedicated support available for enterprise users

Contact: [via GitHub](https://github.com/jbcom)

## ⚡ Response Times

We aim for:

- **Security Issues**: Response within 48 hours
- **Bug Reports**: Triage within 7 days
- **Feature Requests**: Review within 14 days
- **Pull Requests**: Initial review within 7 days

*Note: These are goals, not guarantees. We're a community project and response times may vary.*

## 🌍 Community

Join our growing community:

- **GitHub Stars**: Star the repo to show support
- **Share Your Use Case**: Tell us how you're using SecretSync
- **Contribute**: Code, docs, examples - all contributions welcome!

## 📋 Frequently Asked Questions

### How do I get started?

1. Read the [README](../README.md)
2. Check the [GitHub Actions guide](./GITHUB_ACTIONS.md)
3. Copy an [example workflow](../examples/github-action-workflow.yml)
4. Customize for your needs

### Is SecretSync free?

Yes! SecretSync is free and open source (MIT License).

### Can I use this in production?

Yes! SecretSync is production-ready. Many organizations use it daily.

### How do I upgrade?

For GitHub Actions:
```yaml
# Pin to the current package release tag (recommended)
uses: jbcom/extended-data-library/packages/secretssync@secretssync-v2.0.2

# Pin to an exact package release tag (most stable)
uses: jbcom/extended-data-library/packages/secretssync@secretssync-v2.0.2

# Track the branch tip (not recommended for production)
uses: jbcom/extended-data-library/packages/secretssync@main
```

For CLI:
```bash
# Use go install
go install github.com/jbcom/extended-data-library/packages/secretssync/cmd/secretsync@latest
```

### Where do I report a security issue?

See our [Security Policy](./SECURITY.md) and contact us privately.

### How can I contribute?

See the Contributing section above or open an issue.

## 📝 Feedback

Your feedback helps us improve! Please:

- ⭐ Star the repo if you find it useful
- 🐛 Report bugs when you find them
- 💡 Share feature ideas
- 📖 Improve documentation
- 🗣️ Tell others about SecretSync

## 🔗 Links

- **Repository**: [SecretSync package](https://github.com/jbcom/extended-data-library/tree/main/packages/secretssync)
- **Issues**: [GitHub Issues](https://github.com/jbcom/extended-data-library/issues)
- **Releases**: [Release list](https://github.com/jbcom/extended-data-library/releases)
- **License**: [MIT License](../LICENSE)

---

**Thank you for using SecretSync!** 🚀
