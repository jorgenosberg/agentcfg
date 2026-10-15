---
title: Hook registration
description: How agentcfg registers hook scripts in Claude Code, Codex, and Copilot.
---

Installing a hook script only copies or links the file. To make an agent run it, add a header line to the script's leading comment block, one per registration:

```sh
#!/usr/bin/env bash
# agentcfg-hook: PreToolUse Bash timeout=10
# agentcfg-hook: SessionStart
```

The format is `<Event> [Matcher] [timeout=N]`. Events and matchers use Claude Code's names, and the script receives Claude Code's hook payload on every agent. A script without a header is installed but never registered.

`sync`, `install`, `uninstall`, `toggle`, and `move` update the registrations. `status` lists scripts that are unregistered, registrations that are out of date, and registrations whose script is gone.

## Where registrations go

| Agent | Global | Project |
| --- | --- | --- |
| Claude Code | `settings.json` in the target | `.claude/settings.json`, command uses `"$CLAUDE_PROJECT_DIR"` |
| Codex | `hooks.json` in the target | `.codex/hooks.json`, command uses `git rev-parse --show-toplevel` |
| Copilot CLI | `hooks/agentcfg.json` in the target | `.github/hooks/agentcfg.json` |

In Claude Code and Codex files, agentcfg only touches entries that run a script in the target's own hooks directory, and only when that script has a header or no longer exists. Other entries are left alone. The Copilot file is owned by agentcfg and rewritten in full.

If an entry agentcfg does not own runs a script with the same name as a managed one (for example a Codex entry that points at the copy in `~/.claude/hooks`), `status` and `sync` warn that it may run twice. Remove it by hand.

## Copilot in projects

Copilot CLI also runs the hooks in a project's `.claude/settings.json`. When `move` puts a hook into a project that gets the Claude registration, it skips the Copilot copy so the script does not run twice. The Copilot cloud agent only reads `.github/hooks`, so it does not see those hooks.

## Codex trust

Codex ignores a hook until you review it, and any change to its hooks file resets that. agentcfg writes the file only when the registrations change and prints a reminder to review them with `/hooks` in Codex.
