# syntax=docker/dockerfile:1.7

###
# Build a static secrets-sync binary for the requested platform.
# Tests now run in CI (outside Docker), so this Dockerfile focuses purely
# on compiling and packaging the runtime image.
###
FROM golang:1.26.4-trixie AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG TARGETVARIANT
ARG CGO_ENABLED=0

ARG VERSION=dev

ENV CGO_ENABLED=${CGO_ENABLED} \
    GOTOOLCHAIN=auto
WORKDIR /src

# Update CA certificates
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./

# Cache module and build downloads between runs
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    GOARM=${TARGETVARIANT#v} \
    go build -trimpath \
      -ldflags="-s -w" \
      -o /out/secrets-sync ./cmd/secrets-sync

###
# Runtime image: tiny BusyBox container that only carries the binary and certs.
###
FROM busybox:1.37.0-musl AS runtime

ARG VERSION=dev
ARG SECRETS_SYNC_CONFIG=/etc/secrets-sync/config.yaml

ENV SECRETS_SYNC_CONFIG=${SECRETS_SYNC_CONFIG} \
    SECRETS_SYNC_VERSION=${VERSION}

LABEL org.opencontainers.image.title="secrets-sync" \
      org.opencontainers.image.source="https://github.com/jbcom/secrets-sync" \
      org.opencontainers.image.version=${VERSION}

WORKDIR /app

RUN mkdir -p /etc/ssl/certs
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/secrets-sync /usr/local/bin/secrets-sync

# Default command - Viper reads SECRETS_SYNC_* env vars directly
ENTRYPOINT ["/usr/local/bin/secrets-sync"]
CMD ["pipeline"]
