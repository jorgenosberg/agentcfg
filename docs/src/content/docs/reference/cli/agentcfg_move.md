---
title: agentcfg move
description: agentcfg CLI reference for `agentcfg move`
editUrl: false
---

Move an item between global scope and a project

## Synopsis

Moves an item between your global agent targets and a registered project (see `agentcfg project add`).

--to <project> installs the item into the project for every agent that has it globally, then disables it on those global targets. The item stays in the source tree, so it can be moved back. Project installs are copies unless --link is set, because a symlink into ~/.agentcfg breaks for anyone else who clones the repository.

--to global imports the item from the project into the source tree, installs it on the global targets (-t to limit), and removes the project copies unless --keep is set.

Use --dry-run to see what would change.

```
agentcfg move <item> --to <project|global> [flags]
```

## Options

```
      --dry-run         show what would change without changing anything
      --force           replace existing content at the destination
      --from string     source project when moving to global (default: the only project that has it)
  -h, --help            help for move
      --keep            when moving to global, leave the project copies in place
      --kind string     item kind, to disambiguate when a name exists as several kinds
      --link            symlink into the project instead of copying
      --no-backup       skip the automatic backup
  -t, --target string   global target to install into when moving to global (default: all)
      --to string       destination: a project name or "global"
```

## Options inherited from parent commands

```
      --config string   path to config file (default ~/.agentcfg/config.json)
```

## SEE ALSO

* [agentcfg](../agentcfg/)	 - Sync skills, hooks, and context files across AI agent configs

