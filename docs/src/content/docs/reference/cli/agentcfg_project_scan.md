---
title: agentcfg project scan
description: agentcfg CLI reference for `agentcfg project scan`
editUrl: false
---

Scan project folder(s) for in-repo agent configuration files

## Synopsis

Walks configured project directories and lists all agent-specific files and directories found (CLAUDE.md, .github/copilot-instructions.md, .claude/skills/, .cursorrules, etc.). Provide a project name to scan only that project. Read-only.

```
agentcfg project scan [name] [flags]
```

## Options

```
  -h, --help             help for scan
  -p, --project string   project name (default: all)
```

## Options inherited from parent commands

```
      --config string   path to config file (default ~/.agentcfg/config.json)
```

## SEE ALSO

* [agentcfg project](../agentcfg_project/)	 - Manage project folders to scan for in-repo agent configuration

