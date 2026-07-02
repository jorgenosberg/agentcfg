---
title: Source layout
description: How the agentcfg source tree is organized and how to point it elsewhere.
---

```
<source>/
  skills/<name>/SKILL.md     # skill bundles
  hooks/<name>.sh            # shared hooks
  context/CLAUDE.md          # shared instruction files
  context/AGENTS.md
```

Source path is configurable. Default: `~/.agentcfg/source/`. Point at any existing directory via `agentcfg init --source PATH` or by editing `~/.agentcfg/config.json`.

See the [CLI reference](../../reference/cli/agentcfg_init/) for `init` flags.
