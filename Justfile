default:
    @just --list

# Run Go tests.
test-go *args:
    go test ./... {{ args }}

# Build the Go CLI.
build-go:
    go build -o bin/secrets-sync ./cmd/secrets-sync

# Run Python bridge tests.
test-python *args:
    tox -e py311 -- {{ args }}

# Run Python bridge checks on every supported Python version available locally.
test-python-all:
    tox -e py311,py312,py313,py314

# Lint Python bridge code.
lint-python:
    tox -e lint

# Type-check Python bridge code.
typecheck-python:
    tox -e typecheck

# Build Sphinx docs.
docs:
    tox -e docs

# Run the local CI surface.
ci:
    just test-go
    tox -e lint,typecheck,py311,py312,py313,py314,docs,build
