---
title: agentcfg project
description: agentcfg CLI reference for `agentcfg project`
editUrl: false
---

Manage project folders to scan for in-repo agent configuration

## Synopsis

Project folders are repository or workspace directories that agentcfg scans for agent-specific files such as CLAUDE.md, .github/copilot-instructions.md, .claude/skills/, .cursorrules, and similar artefacts. Scanning is read-only; use `import` to pull items into the global source tree.

## Options

```
  -h, --help   help for project
```

## Options inherited from parent commands

```
      --config string   path to config file (default ~/.agentcfg/config.json)
```

## SEE ALSO

* [agentcfg](../agentcfg/)	 - Sync skills, hooks, and context files across AI agent configs
* [agentcfg project add](../agentcfg_project_add/)	 - Add a project folder
* [agentcfg project list](../agentcfg_project_list/)	 - List configured project folders
* [agentcfg project remove](../agentcfg_project_remove/)	 - Remove a project folder
* [agentcfg project scan](../agentcfg_project_scan/)	 - Scan project folder(s) for in-repo agent configuration files

