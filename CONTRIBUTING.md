# Contributing to SecretSync

Thank you for your interest in contributing to SecretSync! We welcome contributions from the community.

## Code of Conduct

Be respectful, inclusive, and professional. We're all here to learn and improve together.

## How to Contribute

### Reporting Bugs

1. **Search first**: Check if the issue already exists
2. **Use templates**: Follow the bug report template when available
3. **Provide details**: Include configuration (sanitized!), logs, and steps to reproduce
4. **Sanitize secrets**: Never include real credentials or sensitive data

### Suggesting Features

1. **Check existing requests**: Search existing issues first
2. **Describe the use case**: Why is this needed?
3. **Propose a solution**: How should it work?
4. **Consider alternatives**: What workarounds exist today?

### Contributing Code

#### Getting Started

1. **Fork the repository**
   ```bash
   git clone https://github.com/YOUR_USERNAME/secrets-sync.git
   cd secrets-sync
   ```

2. **Create a feature branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

3. **Set up development environment**
   ```bash
   # Install Just if needed
   brew install just

   # Install dependencies
   just deps

   # Verify build
   just build-all

   # Run tests
   just test-go

   # Run docs/lint checks
   just quality
   ```

#### Making Changes

1. **Follow existing code style**
   - Use `gofmt` for formatting
   - Follow Go best practices
   - Add comments for exported functions/types
   - Write tests for new features

2. **Write good commit messages**
   ```
   feat(store): add support for Azure Key Vault
   
   - Implement Azure authentication
   - Add KV client wrapper
   - Include integration tests
   
   Fixes #123
   ```

3. **Add tests**
   - Unit tests for new functions
   - Integration tests for new stores
   - Table-driven tests when appropriate
   - Maintain or improve code coverage

4. **Update documentation**
   - Update README if behavior changes
   - Update relevant docs in `docs/`
   - Add examples if introducing new features
   - Update CHANGELOG.md

#### Testing

```bash
# Run all tests
just test-go

# Run specific package tests
just test-go ./pkg/pipeline

# Run with coverage
just test-unit

# Run with race detection
just test-unit

# Run docs/lint checks
just quality

# Build all release binaries
just build-all

# Verify gopy Python bindings
just python-matrix

# Verify the lower supported Go line
GO_TOOLCHAIN=go1.25.11 just test-go

# Build
just build
```

#### Submitting Changes

1. **Push your changes**
   ```bash
   git push origin feature/your-feature-name
   ```

2. **Create a Pull Request**
   - Use a clear, descriptive title
   - Reference related issues
   - Describe what changed and why
   - Include any breaking changes
   - Add screenshots for UI changes
   - Request review from maintainers

3. **Respond to feedback**
   - Address review comments promptly
   - Update code based on feedback
   - Re-request review when ready

### Pull Request Checklist

- [ ] Code follows project style guidelines
- [ ] Tests added for new features
- [ ] All tests pass locally
- [ ] Documentation updated
- [ ] CHANGELOG.md updated (for user-facing changes)
- [ ] Commits are clear and descriptive
- [ ] No merge conflicts
- [ ] Sanitized any example configs

## Development Guidelines

### Code Style

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` for formatting
- Use meaningful variable names
- Add comments for complex logic
- Keep functions focused and small
- Prefer composition over inheritance

### Testing

- Write table-driven tests when appropriate
- Test both success and error cases
- Use testify for assertions (optional)
- Mock external dependencies
- Test edge cases

Example test structure:
```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"valid input", "test", "result", false},
        {"invalid input", "", "", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := MyFunction(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("MyFunction() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("MyFunction() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Documentation

- Document all exported functions and types
- Include examples in documentation
- Keep README up-to-date
- Update docs/ when adding features
- Add examples to examples/

### Security

- Never commit secrets or credentials
- Sanitize all example configurations
- Use environment variables for sensitive data
- Report security issues privately
- Follow security best practices

## Project Structure

```
secrets-sync/
├── cmd/secrets-sync/           # CLI application
│   ├── cmd/           # Cobra commands
│   └── main.go        # Entry point
├── pkg/
│   ├── client/        # Vault, AWS, and provider clients
│   ├── discovery/     # AWS Organizations and Identity Center discovery
│   ├── driver/        # Supported driver names and validation helpers
│   ├── pipeline/      # Merge, sync, graph, and execution orchestration
│   ├── diff/          # Diff computation and masking
│   └── observability/ # Metrics and request tracking
├── python/            # Optional gopy binding sources
├── docs/              # Documentation
├── examples/          # Example configurations
└── deploy/            # Deployment manifests
```

## Adding a New Secret Backend

To add support for a new backend:

1. **Create a client package**
   ```bash
   mkdir -p pkg/client/newbackend
   ```

2. **Implement the current client shape**
   ```go
   package newbackend
   
   import "github.com/jbcom/secrets-sync/pkg/driver"
   
   type Client struct {
       Name string `yaml:"name,omitempty" json:"name,omitempty"`
   }
   
   func (c *Client) Validate() error {
       if c.Name == "" {
           return driver.ErrPathRequired
       }
       return nil
   }
   
   func (c *Client) Driver() driver.DriverName {
       return driver.DriverName("newbackend")
   }
   ```

3. **Add tests**
   ```go
   package newstore_test
   
   func TestStore_Get(t *testing.T) {
       // test implementation
   }
   ```

4. **Register the backend**
   - Add the driver name in `pkg/driver`
   - Update pipeline config types and validation
   - Add client initialization logic in the pipeline layer
   - Update documentation

5. **Add examples**
   - Create example config in `examples/`
   - Add usage documentation in `docs/`

## Release Process

Releases are managed by maintainers:

1. Update CHANGELOG.md
2. Create version tag
3. Push tag to trigger CI/CD
4. Verify release artifacts
5. Update Marketplace (if applicable)

## Getting Help

- **Documentation**: Read the [docs/](./docs/) directory
- **Issues**: Use [GitHub Issues](https://github.com/jbcom/secrets-sync/issues) for bugs, questions, and feature requests

## License

By contributing to SecretSync, you agree that your contributions will be licensed under the MIT License.

## Recognition

All contributors will be recognized in:
- Git commit history
- Release notes (for significant contributions)
- GitHub contributors page

Thank you for contributing to SecretSync! 🚀
