# Installation

## Requirements

- Go 1.25+
- Docker (optional, for containerized runs or the GitHub Action image)

## Install the CLI

```bash
# Install the latest CLI from the monorepo module path
go install github.com/jbcom/extended-data-library/packages/secretssync/cmd/secretsync@latest
```

## Run with Docker

```bash
docker pull jbcom/secretssync:v1

# Example alias for local CLI-style usage
alias secretsync='docker run --rm -v "$PWD":/workspace -w /workspace jbcom/secretssync:v1'
```

## Build from Source

```bash
git clone https://github.com/jbcom/extended-data-library.git
cd extended-data-library/packages/secretssync
make build

# The compiled binary is written to ./bin/secretsync
./bin/secretsync version
```

## GitHub Action

Use the packaged action from the monorepo subdirectory and pin to a package tag:

```yaml
- uses: jbcom/extended-data-library/packages/secretssync@secretssync-v2.0.1
  with:
    config: config.yaml
```
