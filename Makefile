BINDIR  := bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# Resolve install destination: honour explicit GOBIN, then go env, then ~/go/bin.
GOBIN   ?= $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN   := $(shell go env GOPATH)/bin
endif

LDFLAGS := -s -w \
  -X github.com/jorgenosberg/agentcfg/internal/version.Version=$(VERSION) \
  -X github.com/jorgenosberg/agentcfg/internal/version.Commit=$(COMMIT) \
  -X github.com/jorgenosberg/agentcfg/internal/version.Date=$(DATE)

SANDBOX ?= $(CURDIR)/.sandbox

.PHONY: all build agentcfg lazyagentcfg install uninstall \
        check test vet lint fmt tidy clean run-tui watch \
        sandbox sandbox-cli sandbox-reset gen-docs check-docs

all: build

## build: compile both binaries into ./bin/
build: agentcfg lazyagentcfg

agentcfg:
	@mkdir -p $(BINDIR)
	go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/agentcfg ./cmd/agentcfg

lazyagentcfg:
	@mkdir -p $(BINDIR)
	go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/lazyagentcfg ./cmd/lazyagentcfg

## install: build and install both binaries to $(GOBIN)
install:
	go install -ldflags "$(LDFLAGS)" ./cmd/agentcfg ./cmd/lazyagentcfg
	@echo "installed to $(GOBIN)"

## uninstall: remove installed binaries from $(GOBIN)
uninstall:
	rm -f $(GOBIN)/agentcfg $(GOBIN)/lazyagentcfg

## check: run vet and tests (CI gate)
check: vet test

## test: run all tests
test:
	go test -race -count=1 ./...

## vet: run go vet
vet:
	go vet ./...

## lint: run golangci-lint (install via: brew install golangci-lint)
lint:
	@command -v golangci-lint >/dev/null 2>&1 || \
		{ echo "golangci-lint not found — install: brew install golangci-lint"; exit 1; }
	golangci-lint run ./...

## fmt: format all Go source files
fmt:
	gofmt -s -w .

## tidy: tidy and verify the module graph
tidy:
	go mod tidy
	go mod verify

## gen-docs: regenerate CLI reference Markdown into docs/ from the Cobra command tree
gen-docs:
	go run ./cmd/gendocs

## check-docs: fail if the committed CLI reference is stale vs the Cobra tree
check-docs: gen-docs
	@git diff --exit-code -- docs/src/content/docs/reference/cli || \
		{ echo "CLI reference is stale — run 'make gen-docs' and commit the result"; exit 1; }

## clean: remove build artefacts
clean:
	rm -rf $(BINDIR) dist

## run-tui: build and launch the TUI
run-tui: lazyagentcfg
	./$(BINDIR)/lazyagentcfg

## watch: rebuild and relaunch the TUI on every .go change (requires watchexec)
watch:
	@command -v watchexec >/dev/null 2>&1 || \
		{ echo "watchexec not found — install: brew install watchexec"; exit 1; }
	@mkdir -p bin/.watch
	watchexec -e go -r -- ./scripts/watch-tui.sh

## sandbox: build and run the TUI against an isolated sandbox home (never touches real config)
sandbox: build
	@mkdir -p "$(SANDBOX)"
	@echo "sandbox home: $(SANDBOX)"
	AGENTCFG_HOME="$(SANDBOX)" ./$(BINDIR)/lazyagentcfg

## sandbox-cli: run a CLI command against the sandbox; e.g. make sandbox-cli ARGS="discover"
sandbox-cli: build
	AGENTCFG_HOME="$(SANDBOX)" ./$(BINDIR)/agentcfg $(ARGS)

## sandbox-reset: wipe the sandbox home
sandbox-reset:
	rm -rf "$(SANDBOX)"

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
