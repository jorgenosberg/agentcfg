---
title: agentcfg import
description: agentcfg CLI reference for `agentcfg import`
editUrl: false
---

Copy items from a target's directory into the source tree

## Synopsis

Reads the given target's directory and copies named items into the source tree. Use --all to import everything found. Existing source items are skipped unless --force is set.

```
agentcfg import <target> [item...] [flags]
```

## Options

```
      --all     import every item found in the target
      --force   overwrite source items that already exist
  -h, --help    help for import
```

## Options inherited from parent commands

```
      --config string   path to config file (default ~/.agentcfg/config.json)
```

## SEE ALSO

* [agentcfg](../agentcfg/)	 - Sync skills, hooks, and context files across AI agent configs

