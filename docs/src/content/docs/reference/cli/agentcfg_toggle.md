---
title: agentcfg toggle
description: agentcfg CLI reference for `agentcfg toggle`
editUrl: false
---

Enable or disable an item for one or more targets

## Synopsis

Toggle the disabled state of an item. Without --on or --off, the direction is inferred: if the item is disabled on all specified targets it is enabled; otherwise it is disabled. When targets have mixed state, use -t or --on/--off.

```
agentcfg toggle <item> [flags]
```

## Options

```
  -h, --help            help for toggle
      --off             disable the item
      --on              enable the item
  -t, --target string   target name (default: all)
```

## Options inherited from parent commands

```
      --config string   path to config file (default ~/.agentcfg/config.json)
```

## SEE ALSO

* [agentcfg](../agentcfg/)	 - Sync skills, hooks, and context files across AI agent configs

