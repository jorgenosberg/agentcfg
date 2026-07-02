---
title: Link vs copy
description: The two install strategies agentcfg supports, and when to use each.
---

- **`link`** (default) — symlinks installed entries to the source. Single source of truth, edits propagate instantly. Some agents may not resolve symlinks correctly.
- **`copy`** — snapshots the source into the target. Works with any agent, but requires re-sync after source edits (`agentcfg install <item>` overwrites).

Set the strategy globally via `default_strategy`, or per-target via `strategy`, in `~/.agentcfg/config.json`.

See the [CLI reference](../../reference/cli/agentcfg_target_add/) for setting a target's strategy at creation time.
