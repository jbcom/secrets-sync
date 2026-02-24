# SecretSync Makefile

.PHONY: all build test test-unit test-integration lint lint-fix deps fmt tidy clean help
.PHONY: python-bindings python-install python-clean

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOLINT=golangci-lint

# Python binding parameters
GOPY=gopy
PYTHON=python3
PYTHON_PKG=secretssync
PYTHON_OUTPUT=python/$(PYTHON_PKG)

# Build info
BINARY_NAME=secretsync
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE?=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS=-s -w \
	-X github.com/jbcom/extended-data-library/packages/secretssync/cmd/secretsync/cmd.Version=$(VERSION) \
	-X github.com/jbcom/extended-data-library/packages/secretssync/cmd/secretsync/cmd.Commit=$(COMMIT) \
	-X github.com/jbcom/extended-data-library/packages/secretssync/cmd/secretsync/cmd.Date=$(DATE)

all: lint test build

## Build targets
build:
	@mkdir -p bin
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME) ./cmd/secretsync

## Python bindings (via gopy)
python-bindings:
	@echo "Building Python bindings with gopy..."
	@mkdir -p $(PYTHON_OUTPUT)
	$(GOPY) pkg -output=$(PYTHON_OUTPUT) -vm=$(PYTHON) -name=$(PYTHON_PKG) \
		-version=$(VERSION) \
		-author="Extended Data Library" \
		-email="support@extendeddata.dev" \
		-url="https://github.com/jbcom/extended-data-library/packages/secretssync" \
		-desc="Enterprise-grade secret synchronization pipeline with Python bindings" \
		./python/secretssync
	@echo "Python bindings generated in $(PYTHON_OUTPUT)"

python-build: python-bindings
	@echo "Building Python wheel..."
	cd $(PYTHON_OUTPUT) && $(PYTHON) -m build

python-install: python-bindings
	@echo "Installing Python package locally..."
	cd $(PYTHON_OUTPUT) && make install

python-clean:
	@echo "Cleaning Python bindings..."
	rm -rf $(PYTHON_OUTPUT)/*.so $(PYTHON_OUTPUT)/*.c $(PYTHON_OUTPUT)/*.h \
		$(PYTHON_OUTPUT)/build $(PYTHON_OUTPUT)/dist $(PYTHON_OUTPUT)/*.egg-info \
		$(PYTHON_OUTPUT)/__pycache__ $(PYTHON_OUTPUT)/*.pyc $(PYTHON_OUTPUT)/*.pyo

## Test targets
test: test-unit

test-unit:
	$(GOTEST) -race -coverprofile=coverage.out ./...

# Integration tests require LocalStack + Vault
# Either run manually with docker-compose or let CI handle it
test-integration:
	@echo "Running integration tests..."
	@if [ -z "$$VAULT_ADDR" ] || [ -z "$$AWS_ENDPOINT_URL" ]; then \
		echo "Starting test environment with docker-compose..."; \
		docker-compose -f docker-compose.test.yml up --abort-on-container-exit --exit-code-from test-runner; \
	else \
		echo "Using existing environment (VAULT_ADDR=$$VAULT_ADDR, AWS_ENDPOINT_URL=$$AWS_ENDPOINT_URL)"; \
		$(GOTEST) -v -tags=integration ./tests/integration/...; \
	fi

# Run integration tests with docker-compose (always starts fresh)
test-integration-docker:
	docker-compose -f docker-compose.test.yml down -v 2>/dev/null || true
	docker-compose -f docker-compose.test.yml up --build --abort-on-container-exit --exit-code-from test-runner
	docker-compose -f docker-compose.test.yml down -v

# Start the test environment (for manual testing)
test-env-up:
	docker-compose -f docker-compose.test.yml up -d localstack vault
	@echo "Waiting for services to be healthy..."
	@for i in 1 2 3 4 5 6 7 8 9 10 11 12; do \
		if docker-compose -f docker-compose.test.yml ps | grep -q "(healthy)" 2>/dev/null; then \
			echo "Services are healthy!"; \
			break; \
		fi; \
		if [ $$i -eq 12 ]; then \
			echo "Warning: Services may not be fully healthy, proceeding anyway"; \
		else \
			echo "Waiting... ($$i/12)"; \
			sleep 5; \
		fi; \
	done
	@echo ""
	@echo "Test environment ready. Export these variables:"
	@echo "  export VAULT_ADDR=http://localhost:8200"
	@echo "  export VAULT_TOKEN=test-root-token"
	@echo "  export AWS_ENDPOINT_URL=http://localhost:4566"
	@echo "  export AWS_ACCESS_KEY_ID=test"
	@echo "  export AWS_SECRET_ACCESS_KEY=test"
	@echo "  export AWS_REGION=us-east-1"
	@echo ""
	@echo "Then run: make test-integration"

test-env-down:
	docker-compose -f docker-compose.test.yml down -v

## Lint targets
lint:
	$(GOLINT) run

lint-fix:
	$(GOLINT) run --fix

## Formatting
fmt:
	$(GOFMT) ./...

## Dependency management
tidy:
	$(GOMOD) tidy

deps:
	$(GOMOD) download
	$(GOMOD) tidy

## Clean targets
clean: python-clean
	rm -f $(BINARY_NAME)
	rm -rf bin/
	rm -f coverage.out
	docker-compose -f docker-compose.test.yml down -v 2>/dev/null || true

## Help
help:
	@echo "Available targets:"
	@echo ""
	@echo "Go targets:"
	@echo "  build                 - Build the binary to bin/"
	@echo "  test                  - Run unit tests with race detection and coverage"
	@echo "  test-unit             - Run unit tests with race detection and coverage"
	@echo "  test-integration      - Run integration tests (auto-detects environment)"
	@echo "  test-integration-docker - Run integration tests via docker-compose"
	@echo "  test-env-up           - Start LocalStack + Vault for local testing"
	@echo "  test-env-down         - Stop test environment"
	@echo "  lint                  - Run golangci-lint"
	@echo "  lint-fix              - Run golangci-lint with auto-fix"
	@echo "  fmt                   - Format Go code with go fmt"
	@echo "  tidy                  - Run go mod tidy"
	@echo "  deps                  - Download and tidy dependencies"
	@echo "  clean                 - Clean build artifacts and test containers"
	@echo ""
	@echo "Python binding targets:"
	@echo "  python-bindings       - Generate Python bindings via gopy"
	@echo "  python-build          - Build Python wheel package"
	@echo "  python-install        - Install Python package locally"
	@echo "  python-clean          - Clean Python build artifacts"
