---
title: agentcfg backup prune
description: agentcfg CLI reference for `agentcfg backup prune`
editUrl: false
---

## agentcfg backup prune

Delete old snapshots, keeping the most recent N

```
agentcfg backup prune [flags]
```

### Options

```
  -h, --help       help for prune
      --keep int   number of most recent snapshots to keep (default 5)
```

### Options inherited from parent commands

```
      --config string   path to config file (default ~/.agentcfg/config.json)
```

### SEE ALSO

* [agentcfg backup](../agentcfg_backup/)	 - Manage snapshots of target directories

