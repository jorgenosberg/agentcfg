---
title: agentcfg backup restore
description: agentcfg CLI reference for `agentcfg backup restore`
editUrl: false
---

Restore a snapshot back to original target paths

```
agentcfg backup restore [flags]
```

## Options

```
  -h, --help        help for restore
      --index int   restore snapshot by 1-based index (as shown by 'backup list')
      --latest      restore from the most recent snapshot without prompting
```

## Options inherited from parent commands

```
      --config string   path to config file (default ~/.agentcfg/config.json)
```

## SEE ALSO

* [agentcfg backup](../agentcfg_backup/)	 - Manage snapshots of target directories

