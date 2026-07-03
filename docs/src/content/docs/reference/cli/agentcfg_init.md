---
title: agentcfg init
description: agentcfg CLI reference for `agentcfg init`
editUrl: false
---

Write a default config file

## Synopsis

Write a starter config at the configured path (default ~/.agentcfg/config.json) and create the source tree skeleton.

When stdin is a terminal the command launches an interactive wizard that discovers installed agents, lets you register targets, and optionally imports items into the source tree. Pass --no-interactive to skip the wizard and write a bare config file.

```
agentcfg init [flags]
```

## Options

```
  -h, --help             help for init
      --no-interactive   write a bare config file without the setup wizard
      --source string    path to source tree (default ~/.agentcfg/source)
```

## Options inherited from parent commands

```
      --config string   path to config file (default ~/.agentcfg/config.json)
```

## SEE ALSO

* [agentcfg](../agentcfg/)	 - Sync skills, hooks, and context files across AI agent configs

