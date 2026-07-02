---
title: agentcfg backup
description: agentcfg CLI reference for `agentcfg backup`
editUrl: false
---

## agentcfg backup

Manage snapshots of target directories

```
agentcfg backup [flags]
```

### Options

```
  -h, --help   help for backup
```

### Options inherited from parent commands

```
      --config string   path to config file (default ~/.agentcfg/config.json)
```

### SEE ALSO

* [agentcfg](../agentcfg/)	 - Sync skills, hooks, and context files across AI agent configs
* [agentcfg backup create](../agentcfg_backup_create/)	 - Snapshot all target directories now
* [agentcfg backup list](../agentcfg_backup_list/)	 - List available snapshots
* [agentcfg backup prune](../agentcfg_backup_prune/)	 - Delete old snapshots, keeping the most recent N
* [agentcfg backup restore](../agentcfg_backup_restore/)	 - Restore a snapshot back to original target paths

