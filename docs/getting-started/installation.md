# Installation

## Requirements

- Go 1.25+
- Docker (optional, for containerized runs or the GitHub Action image)

## Install the CLI

```bash
# Install the latest CLI from the standalone module path
go install github.com/jbcom/secrets-sync/cmd/secretsync@latest
```

## Run with Docker

```bash
docker pull jbcom/secretssync:v1

# Example alias for local CLI-style usage
alias secretsync='docker run --rm -v "$PWD":/workspace -w /workspace jbcom/secretssync:v1'
```

## Build from Source

```bash
git clone https://github.com/jbcom/secrets-sync.git
cd secrets-sync
make build

# The compiled binary is written to ./bin/secretsync
./bin/secretsync version
```

## GitHub Action

Use the packaged action from this standalone repository and pin to a release
tag. Replace `X.Y.Z` with a published release version:

```yaml
- uses: jbcom/secrets-sync@secrets-sync-vX.Y.Z
  with:
    config: config.yaml
```
