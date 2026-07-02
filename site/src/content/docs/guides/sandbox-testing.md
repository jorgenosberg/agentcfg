---
title: Sandbox testing
description: Try agentcfg commands against an isolated home directory without touching your real agent configs.
---

Set `AGENTCFG_HOME` to any directory and agentcfg will read and write only under that path. This means its own state (`~/.agentcfg/*`), the agent catalog (`~/.claude`, `~/.codex`, etc.), and Claude plugin reads/writes are all redirected, so your real local AI agent configs are not touched.

```sh
make sandbox              # build + launch TUI with AGENTCFG_HOME=./.sandbox
make sandbox-cli ARGS="init"
make sandbox-cli ARGS="discover"
make sandbox-reset        # wipe the sandbox
```

Or run directly:

```sh
AGENTCFG_HOME=/tmp/agentcfg-test agentcfg init
AGENTCFG_HOME=/tmp/agentcfg-test lazyagentcfg
```

To exercise discover/sync with fake agent dirs, create them inside the sandbox home:

```sh
mkdir -p .sandbox/.claude/skills .sandbox/.codex
make sandbox-cli ARGS="discover"
```
