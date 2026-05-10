BINDIR := bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
  -X github.com/jorgenosberg/agentcfg/internal/version.Version=$(VERSION) \
  -X github.com/jorgenosberg/agentcfg/internal/version.Commit=$(COMMIT) \
  -X github.com/jorgenosberg/agentcfg/internal/version.Date=$(DATE)

.PHONY: all build agentcfg lazyagentcfg tidy test fmt vet clean run-tui

all: build

build: agentcfg lazyagentcfg

agentcfg:
	@mkdir -p $(BINDIR)
	go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/agentcfg ./cmd/agentcfg

lazyagentcfg:
	@mkdir -p $(BINDIR)
	go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/lazyagentcfg ./cmd/lazyagentcfg

tidy:
	go mod tidy

test:
	go test ./...

fmt:
	gofmt -s -w .

vet:
	go vet ./...

clean:
	rm -rf $(BINDIR) dist

run-tui: lazyagentcfg
	./$(BINDIR)/lazyagentcfg
