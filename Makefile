# cwpromql — developer tasks
BINARY   := cwpromql
PKG      := ./...
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X github.com/lewinkedrs/cw-otel-cli/internal/cli.version=$(VERSION)

# Corp networks often block the default Go module proxy.
export GOPROXY ?= direct
export GOSUMDB ?= off

.PHONY: all build install test cover vet fmt fmt-check lint tidy clean

all: fmt-check vet test build

build: ## Build the binary into ./bin
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) .

install: ## Install into $(go env GOPATH)/bin
	go install -ldflags "$(LDFLAGS)" .

test: ## Run unit tests
	go test $(PKG)

cover: ## Run tests with a coverage summary
	go test $(PKG) -cover

vet: ## go vet
	go vet $(PKG)

fmt: ## Format the code
	gofmt -w .

fmt-check: ## Fail if any file is not gofmt-clean
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

tidy: ## Tidy modules
	go mod tidy

clean: ## Remove build artifacts
	rm -rf bin

# Live smoke test against a real account (needs AWS creds):
#   make live REGION=us-east-2
.PHONY: live
live:
	CWPROMQL_LIVE=1 go test ./internal/promql -run TestLive -v
