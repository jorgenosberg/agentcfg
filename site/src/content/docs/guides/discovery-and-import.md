---
title: Discovery & import
description: How agentcfg finds existing agent directories and imports their contents into your source tree.
---

agentcfg ships a built-in catalog of known AI agent install directories: `~/.claude`, `~/.codex`, `~/.copilot`, `~/.gemini` (also used by the Antigravity CLI), `~/.config/opencode`, `~/.agents`. Discovery is opt-in:

- `init` does not scan anywhere outside the source directory.
- `discover` lists items in those catalog paths *only when they already exist on disk*, and only when you run the command explicitly.
- Registration as a target requires `--add <name>` or `--add-all`.

Use `import` to copy items found on a target back into the agentcfg source tree, so they become the single source of truth:

```sh
agentcfg import claude --all           # copy every item from claude into source
agentcfg import codex CLAUDE.md        # copy a single item
```

See the [CLI reference](../../reference/cli/agentcfg_discover/) for every `discover` and `import` flag.
