---
title: Installation
description: Install agentcfg and lazyagentcfg via Homebrew, go install, or from source.
---

agentcfg ships two binaries:

- `agentcfg` — the CLI.
- `lazyagentcfg` — an interactive TUI (lazygit-style).

## Homebrew

```sh
brew install jorgenosberg/tap/agentcfg       # CLI
brew install jorgenosberg/tap/lazyagentcfg   # TUI
```

## go install

```sh
go install github.com/jorgenosberg/agentcfg/cmd/agentcfg@latest
go install github.com/jorgenosberg/agentcfg/cmd/lazyagentcfg@latest
```

Requires Go 1.24 or newer.

## From source

```sh
git clone https://github.com/jorgenosberg/agentcfg
cd agentcfg
make build
```

Binaries land in `./bin/`.
