---
title: agentcfg sync
description: agentcfg CLI reference for `agentcfg sync`
editUrl: false
---

## agentcfg sync

Install all absent and drifted items across all targets

```
agentcfg sync [flags]
```

### Options

```
      --dry-run         show what would be installed without making changes
      --force           adopt unmanaged files (replace existing files not managed by agentcfg)
  -h, --help            help for sync
      --no-backup       skip automatic backup before syncing
  -t, --target string   target name (default: all)
```

### Options inherited from parent commands

```
      --config string   path to config file (default ~/.agentcfg/config.json)
```

### SEE ALSO

* [agentcfg](../agentcfg/)	 - Sync skills, hooks, and context files across AI agent configs

