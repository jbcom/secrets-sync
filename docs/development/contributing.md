# Contributing

Thank you for your interest in contributing to SecretSync.

## Development Setup

```bash
git clone https://github.com/jbcom/extended-data-library.git
cd extended-data-library/packages/secretssync

# Download Go dependencies
go mod download
```

## Running Tests

```bash
# Unit tests
go test ./...

# Race-enabled unit tests with coverage output
make test

# Integration tests (starts local test services via docker-compose)
make test-integration-docker
```

## Code Style

This project uses:
- `gofmt` for formatting
- `golangci-lint` for linting
- GoDoc comments for exported APIs

```bash
gofmt ./...
golangci-lint run
```

## Building and Bindings

```bash
# Build the CLI
make build

# Generate Python bindings
make python-bindings
```

## Pull Request Process

1. Create a feature branch from `main`
2. Make your changes with tests
3. Ensure lint and tests pass locally
4. Submit a PR against `jbcom/extended-data-library`

## Commit Messages

Use conventional commits:
- `feat:` New features
- `fix:` Bug fixes
- `docs:` Documentation changes
- `refactor:` Code refactoring
- `test:` Test changes
- `chore:` Maintenance tasks
