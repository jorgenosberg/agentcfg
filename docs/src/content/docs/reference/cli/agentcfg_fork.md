---
title: agentcfg fork
description: agentcfg CLI reference for `agentcfg fork`
editUrl: false
---

Fork a Claude plugin into the agentcfg-owned marketplace

## Synopsis

fork copies a Claude Code plugin's entire bundle into the agentcfg fork marketplace (~/.agentcfg/forks/), registers it with Claude Code, disables the upstream plugin, and enables the fork. You own the copy and can edit any file in it directly.

Pass --claude-dir (repeatable) to register the fork with more than one Claude Code home (e.g. a second account's directory); the bundle is still copied only once and shared across all of them. Defaults to ~/.claude when omitted.

Use the `list` and `status` subcommands to inspect recorded forks.

```
agentcfg fork <plugin@marketplace> [flags]
```

## Options

```
      --claude-dir stringArray   Claude Code home directory to fork into (repeatable); defaults to ~/.claude
      --dry-run                  print what would be forked without making changes
  -h, --help                     help for fork
```

## Options inherited from parent commands

```
      --config string   path to config file (default ~/.agentcfg/config.json)
```

## SEE ALSO

* [agentcfg](../agentcfg/)	 - Sync skills, hooks, and context files across AI agent configs
* [agentcfg fork list](../agentcfg_fork_list/)	 - List recorded plugin forks
* [agentcfg fork status](../agentcfg_fork_status/)	 - Check whether upstream plugins have advanced past the forked version

