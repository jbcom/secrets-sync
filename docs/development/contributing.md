# Contributing

Thank you for your interest in contributing to SecretSync.

## Development Setup

```bash
git clone https://github.com/jbcom/secrets-sync.git
cd secrets-sync

# Download Go dependencies
go mod download
```

## Running Tests

```bash
# Vulnerability scan
go run golang.org/x/vuln/cmd/govulncheck@v1.3.0 ./...

# Unit tests
go test ./...

# Race-enabled unit tests with coverage output
just test-unit

# Integration tests (starts local test services via docker-compose)
just test-integration-docker
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

## Building

```bash
# Build the CLI
just build
```

## Pull Request Process

1. Create a feature branch from `main`
2. Make your changes with tests
3. Ensure lint and tests pass locally
4. Submit a PR against `jbcom/secrets-sync`

## Commit Messages

Use conventional commits:
- `feat:` New features
- `fix:` Bug fixes
- `docs:` Documentation changes
- `refactor:` Code refactoring
- `test:` Test changes
- `chore:` Maintenance tasks
