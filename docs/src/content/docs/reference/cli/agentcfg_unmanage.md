---
title: agentcfg unmanage
description: agentcfg CLI reference for `agentcfg unmanage`
editUrl: false
---

Return an item to the target dir as a real file and stop managing it

## Synopsis

Removes the symlink (or managed copy) at the destination and writes a real copy of the file from source. The item remains in the agentcfg source tree and is added to the target's disabled list so future syncs leave it alone.

```
agentcfg unmanage <item> [flags]
```

## Options

```
  -h, --help            help for unmanage
  -t, --target string   target name (default: all)
```

## Options inherited from parent commands

```
      --config string   path to config file (default ~/.agentcfg/config.json)
```

## SEE ALSO

* [agentcfg](../agentcfg/)	 - Sync skills, hooks, and context files across AI agent configs

