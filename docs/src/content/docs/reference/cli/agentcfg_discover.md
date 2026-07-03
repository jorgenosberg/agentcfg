---
title: agentcfg discover
description: agentcfg CLI reference for `agentcfg discover`
editUrl: false
---

List known AI agent install dirs and items found in them

## Synopsis

Walk the built-in catalog of known agent install paths under $HOME and list items found in each. Read-only by default. Use --add <name> (repeatable) or --add-all to register discovered agents as targets in the config. Use --paths to print which paths the catalog checks without scanning.

Use --path <dir> to scan a custom directory instead of the catalog. Supply --as <agent-type> to apply that agent's layout defaults.

```
agentcfg discover [flags]
```

## Options

```
      --add strings   register named discovered agent as target (repeatable)
      --add-all       register every discovered agent as a target
      --as string     agent type for --path (claude, codex, etc.)
  -h, --help          help for discover
      --path string   scan this directory instead of the built-in catalog
      --paths         print catalog paths without scanning, then exit
```

## Options inherited from parent commands

```
      --config string   path to config file (default ~/.agentcfg/config.json)
```

## SEE ALSO

* [agentcfg](../agentcfg/)	 - Sync skills, hooks, and context files across AI agent configs

