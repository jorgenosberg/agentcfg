---
title: Quick start
description: Initialize agentcfg, discover agent directories, import existing config, and install it everywhere.
---

```sh
agentcfg init                          # writes config + creates source skeleton
agentcfg discover                      # list known agent dirs found in $HOME
agentcfg discover --paths              # show which paths the catalog checks
agentcfg discover --add claude         # register a discovered agent as a target
agentcfg discover --add-all            # register every discovered agent

agentcfg import claude --all           # copy items from claude into source
agentcfg import codex CLAUDE.md        # copy a single item

agentcfg status                        # show install state across targets
agentcfg install obsidian-cli          # install into every target
agentcfg install obsidian-cli -t claude
```

See [Discovery & import](../../guides/discovery-and-import/) and [Source layout](../../guides/source-layout/) for the concepts behind these commands, or the [CLI reference](../../reference/cli/agentcfg/) for every flag.
