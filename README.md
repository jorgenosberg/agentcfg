# agentcfg

Sync skills, hooks, and instruction files (CLAUDE.md, AGENTS.md, etc.)
across local AI coding agent configurations from one user-defined source
tree.

Two binaries:

- `agentcfg` — scriptable CLI.
- `lazyagentcfg` — interactive TUI (lazygit-style).

## Status

Early scaffold. Not yet usable.

## Install

```sh
brew install jorgenosberg/tap/agentcfg
```

Tap not yet published.

## Quick start

```sh
agentcfg init --source ~/.ai          # or omit --source to use ~/.agentcfg/source
agentcfg target add claude   ~/.claude
agentcfg target add codex    ~/.codex
agentcfg target add opencode ~/.config/opencode
agentcfg status
agentcfg install obsidian-cli         # install into all targets
agentcfg install obsidian-cli -t claude
```

## Source layout

```
<source>/
  skills/<name>/SKILL.md     # skill bundles
  hooks/<name>.sh            # shared hooks
  context/CLAUDE.md          # shared instruction files
  context/AGENTS.md
```

Source path is configurable. Default: `~/.agentcfg/source/`. Point at any
existing directory via `agentcfg init --source PATH` or by editing
`~/.agentcfg/config.json`.

## Strategy: link vs copy

- `link` (default) — symlinks installed entries to the source. Single
  source of truth, edits propagate instantly. Some agents may not resolve
  symlinks correctly.
- `copy` — snapshots the source into the target. Works with any agent but
  requires re-sync after source edits (`agentcfg install <item>`
  overwrites).

Set globally via `default_strategy` or per-target via `strategy` in the
config file.

## Build

```sh
make build
```
