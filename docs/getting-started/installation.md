# Installation

## Requirements

- Go 1.25.11 or Go 1.26.4 for source builds; CI validates both supported
  upstream Go lines at known non-vulnerable patch levels.
- Docker (optional, for containerized runs or the GitHub Action image)

The Justfile defaults maintainer builds and generated docs to Go 1.26.4. To
verify the lower supported line locally, prefix commands with
`GO_TOOLCHAIN=go1.25.11`, for example `GO_TOOLCHAIN=go1.25.11 just test-go`.

## Install the CLI

```bash
# Install the latest CLI from the standalone module path
go install github.com/jbcom/secrets-sync/cmd/secrets-sync@latest
```

## Run with Docker

```bash
docker pull ghcr.io/jbcom/secrets-sync:v2.3.1

# Example alias for local CLI-style usage
alias secrets-sync='docker run --rm -v "$PWD":/workspace -w /workspace ghcr.io/jbcom/secrets-sync:v2.3.1'
```

## Build from Source

```bash
git clone https://github.com/jbcom/secrets-sync.git
cd secrets-sync
brew install just # macOS; use apt/dnf/pacman equivalent on Linux
just build

# The compiled binary is written to ./bin/secrets-sync
./bin/secrets-sync version
```

## GitHub Action

Use the packaged action from this standalone repository and pin to a release
tag. Replace `X.Y.Z` with a published release version:

```yaml
- uses: jbcom/secrets-sync@vX.Y.Z
  with:
    config: config.yaml
```
