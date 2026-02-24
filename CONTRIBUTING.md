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

1. **Check existing requests**: Search issues and discussions
2. **Describe the use case**: Why is this needed?
3. **Propose a solution**: How should it work?
4. **Consider alternatives**: What workarounds exist today?

### Contributing Code

#### Getting Started

1. **Fork the repository**
   ```bash
   git clone https://github.com/YOUR_USERNAME/secretsync.git
   cd secretsync
   ```

2. **Create a feature branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

3. **Set up development environment**
   ```bash
   # Install dependencies
   go mod download
   
   # Verify build
   go build ./...
   
   # Run tests
   go test ./...
   
   # Run linter
   golangci-lint run
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
go test ./...

# Run specific package tests
go test ./pkg/pipeline

# Run with coverage
go test -cover ./...

# Run with race detection
go test -race ./...

# Lint code
golangci-lint run

# Build
go build ./...
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
secretsync/
├── cmd/secretsync/           # CLI application
│   ├── cmd/           # Cobra commands
│   └── main.go        # Entry point
├── pkg/               # Public packages
│   ├── pipeline/      # Pipeline orchestration
│   ├── diff/          # Diff computation
│   └── ...
├── stores/            # Secret store implementations
│   ├── vault/         # Vault store
│   ├── aws/           # AWS Secrets Manager
│   └── ...
├── internal/          # Private packages
├── docs/              # Documentation
├── examples/          # Example configurations
└── deploy/            # Deployment manifests
```

## Adding a New Secret Store

To add support for a new secret store:

1. **Create store package**
   ```bash
   mkdir -p stores/newstore
   ```

2. **Implement Store interface**
   ```go
   package newstore
   
   import "github.com/extended-data-library/secretssync/pkg/store"
   
   type Store struct {
       // configuration fields
   }
   
   func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
       // implementation
   }
   
   func (s *Store) Set(ctx context.Context, key string, value []byte) error {
       // implementation
   }
   
   func (s *Store) List(ctx context.Context, prefix string) ([]string, error) {
       // implementation
   }
   ```

3. **Add tests**
   ```go
   package newstore_test
   
   func TestStore_Get(t *testing.T) {
       // test implementation
   }
   ```

4. **Register store**
   - Update pipeline config to include new store
   - Add store initialization logic
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
- **Discussions**: Use [GitHub Discussions](https://github.com/extended-data-library/secretssync/discussions)
- **Issues**: For bugs and features, use [GitHub Issues](https://github.com/extended-data-library/secretssync/issues)

## License

By contributing to SecretSync, you agree that your contributions will be licensed under the MIT License.

## Recognition

All contributors will be recognized in:
- Git commit history
- Release notes (for significant contributions)
- GitHub contributors page

Thank you for contributing to SecretSync! 🚀
