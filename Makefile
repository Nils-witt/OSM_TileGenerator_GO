# Build, test and lint shortcuts. Run `make help` for the target list.
# The test, lint and audit targets mirror .github/workflows/ci.yml.

BINARY_NAME   := osm-tile-generator
MODULE        := github.com/Nils-witt/OSM_TileGenerator_GO
CMD           := ./cmd/$(BINARY_NAME)
GO            ?= go
GOLANGCI_LINT ?= golangci-lint
GOVULNCHECK   ?= govulncheck
GORELEASER    ?= goreleaser

# Same version metadata GoReleaser injects (see .goreleaser.yaml).
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X $(MODULE)/internal/version.Version=$(VERSION) \
	-X $(MODULE)/internal/version.Commit=$(COMMIT) \
	-X $(MODULE)/internal/version.Date=$(DATE)

# Arguments passed to the binary by `make run`, e.g. make run ARGS="serve -port 9000".
ARGS ?= generate

.DEFAULT_GOAL := build

.PHONY: all build install run clean test test-short cover vet lint lint-fix fmt \
	tidy tidy-check audit ci release-snapshot help

## all: Lint, test and build
all: lint test build

## build: Build bin/osm-tile-generator with version metadata
build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME) $(CMD)

## install: Install the binary into $GOBIN with version metadata
install:
	$(GO) install -trimpath -ldflags "$(LDFLAGS)" $(CMD)

## run: Build and run the binary with ARGS (defaults to generate)
run: build
	./bin/$(BINARY_NAME) $(ARGS)

## clean: Remove build output, coverage files and GoReleaser dist/
clean:
	rm -rf bin/ dist/ coverage.out coverage.html

## test: Run all tests with the race detector, as CI does (~2 min)
test:
	$(GO) test -race -shuffle=on -coverprofile=coverage.out ./...

## test-short: Run tests in -short mode
test-short:
	$(GO) test -short -race ./...

## cover: Run tests and write an HTML coverage report to coverage.html
cover: test
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## vet: Run go vet
vet:
	$(GO) vet ./...

## lint: Run golangci-lint with .golangci.yml
lint:
	$(GOLANGCI_LINT) run ./...

## lint-fix: Run golangci-lint and apply automatic fixes
lint-fix:
	$(GOLANGCI_LINT) run --fix ./...

## fmt: Format code with the configured formatters (gofumpt, goimports)
fmt:
	$(GOLANGCI_LINT) fmt ./...

## tidy: Tidy go.mod and go.sum
tidy:
	$(GO) mod tidy

## tidy-check: Fail if go.mod or go.sum are not tidy
tidy-check:
	$(GO) mod tidy
	git diff --exit-code go.mod go.sum

## audit: Scan for known vulnerabilities reachable from the code
audit:
	$(GOVULNCHECK) ./...

## ci: Run every check CI runs
ci:
	$(GO) mod verify
	$(MAKE) tidy-check vet test lint audit

## release-snapshot: Build release archives locally without publishing
release-snapshot:
	$(GORELEASER) release --snapshot --clean

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/  /'
