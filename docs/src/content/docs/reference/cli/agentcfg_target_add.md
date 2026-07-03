---
title: agentcfg target add
description: agentcfg CLI reference for `agentcfg target add`
editUrl: false
---

Add a sync target

```
agentcfg target add <name> <path> [flags]
```

## Options

```
      --agent string      agent type profile: claude, codex, copilot, gemini, cursor, cline, windsurf, aider, agents, opencode
      --alias string      group alias (e.g. 'claude' to group multiple Claude targets together)
  -h, --help              help for add
      --strategy string   link or copy (default: config default)
```

## Options inherited from parent commands

```
      --config string   path to config file (default ~/.agentcfg/config.json)
```

## SEE ALSO

* [agentcfg target](../agentcfg_target/)	 - Manage sync targets

