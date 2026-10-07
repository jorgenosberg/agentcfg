---
title: Installation
description: Install agentcfg via Homebrew, go install, or from source.
---

agentcfg ships as a single binary. Run `agentcfg` with no arguments in a terminal to open the interactive TUI, or use any subcommand as a scriptable CLI.

## Homebrew

```sh
brew install jorgenosberg/tap/agentcfg
```

## go install

```sh
go install github.com/jorgenosberg/agentcfg/cmd/agentcfg@latest
```

Requires Go 1.24 or newer.

## From source

```sh
git clone https://github.com/jorgenosberg/agentcfg
cd agentcfg
make build
```

Binaries land in `./bin/`.
