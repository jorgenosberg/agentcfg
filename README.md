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
agentcfg init                          # writes config + creates source skeleton
agentcfg discover                      # list known agent dirs found in $HOME
agentcfg discover --paths              # show which paths the catalog checks
agentcfg discover --add claude         # register a discovered agent as a target
agentcfg discover --add-all            # register every discovered agent

agentcfg import claude --all           # copy items from claude into source
agentcfg import codex CLAUDE.md        # copy a single item

agentcfg status                        # show install state across targets
agentcfg install obsidian-cli          # install into every target
agentcfg install obsidian-cli -t claude
```

## Discovery and import

agentcfg ships a built-in catalog of known AI agent install directories:
`~/.claude`, `~/.codex`, `~/.config/opencode`, `~/.config/github-copilot`,
`~/.agents`. Discovery is opt-in:

- `init` does not scan anywhere outside the source directory.
- `discover` lists items in those catalog paths *only when they already
  exist on disk* and you run the command explicitly.
- Registration as a target requires `--add <name>` or `--add-all`.

Use `import` to copy items found on a target back into the agentcfg
source tree, so they become the single source of truth.

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
