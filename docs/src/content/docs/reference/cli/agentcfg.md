---
title: agentcfg
description: agentcfg CLI reference for `agentcfg`
editUrl: false
---

Sync skills, hooks, and context files across AI agent configs

## Synopsis

agentcfg keeps a single source-of-truth directory in sync with one or more AI coding agent directories (Claude Code, Codex, Copilot, opencode, ...). Source path is user-configurable; default is ~/.agentcfg/source.

Set AGENTCFG_HOME to any directory to run in an isolated sandbox — all reads and writes (state, catalog, Claude plugin files) go there instead of the real $HOME.

## Options

```
      --config string   path to config file (default ~/.agentcfg/config.json)
  -h, --help            help for agentcfg
```

## SEE ALSO

* [agentcfg backup](../agentcfg_backup/)	 - Manage snapshots of target directories
* [agentcfg discover](../agentcfg_discover/)	 - List known AI agent install dirs and items found in them
* [agentcfg edit](../agentcfg_edit/)	 - Open a source item in your editor
* [agentcfg fork](../agentcfg_fork/)	 - Fork a Claude plugin into the agentcfg-owned marketplace
* [agentcfg import](../agentcfg_import/)	 - Copy items from a target's directory into the source tree
* [agentcfg init](../agentcfg_init/)	 - Write a default config file
* [agentcfg install](../agentcfg_install/)	 - Install an item into one or more targets
* [agentcfg list](../agentcfg_list/)	 - List items in the source tree
* [agentcfg project](../agentcfg_project/)	 - Manage project folders to scan for in-repo agent configuration
* [agentcfg status](../agentcfg_status/)	 - Show install state of every item across every target
* [agentcfg sync](../agentcfg_sync/)	 - Install all absent and drifted items across all targets
* [agentcfg target](../agentcfg_target/)	 - Manage sync targets
* [agentcfg toggle](../agentcfg_toggle/)	 - Enable or disable an item for one or more targets
* [agentcfg uninstall](../agentcfg_uninstall/)	 - Remove an item from one or more targets
* [agentcfg unmanage](../agentcfg_unmanage/)	 - Return an item to the target dir as a real file and stop managing it
* [agentcfg version](../agentcfg_version/)	 - Manage saved versions of source items

