// Package catalog lists well-known AI agent install directories.
//
// agentcfg uses this to power the `discover` command. Registration as a
// target is always explicit via `--add`.
//
// Sources (verified 2026-05):
//   - Claude Code:  ~/.claude         (skills/, hooks/, CLAUDE.md)
//   - Codex CLI:    ~/.codex          (skills/, AGENTS.md, AGENTS.override.md)
//   - Copilot CLI:  ~/.copilot        (settings.json, mcp-config.json)
//   - Gemini CLI:   ~/.gemini         (extensions/, GEMINI.md)
//   - opencode:     ~/.config/opencode (skills/, AGENTS.md)
//   - agents (opencode/Claude compat): ~/.agents (skills/, AGENTS.md)
package catalog

import (
	"os"
	"path/filepath"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/paths"
	"github.com/jorgenosberg/agentcfg/internal/source"
)

// KnownAgents returns the built-in catalog of agent install layouts.
//
// A missing kind key in Subdirs means: do not scan that kind for this agent.
// Context with subdir "" means: scan the agent root for *.md files.
func KnownAgents() []config.Target {
	home, err := paths.Home()
	if err != nil {
		return nil
	}

	return []config.Target{
		{
			Name:  "claude",
			Path:  filepath.Join(home, ".claude"),
			Agent: "claude",
			Alias: "claude",
			Subdirs: map[string]string{
				source.KindSkill:   "skills",
				source.KindHook:    "hooks",
				source.KindContext: "",
			},
		},
		{
			Name:  "codex",
			Path:  filepath.Join(home, ".codex"),
			Agent: "codex",
			Alias: "codex",
			Subdirs: map[string]string{
				source.KindSkill:   "skills",
				source.KindContext: "",
			},
		},
		{
			Name:  "copilot",
			Path:  filepath.Join(home, ".copilot"),
			Agent: "copilot",
			Alias: "copilot",
			Subdirs: map[string]string{
				source.KindContext: "",
			},
		},
		{
			Name:  "gemini",
			Path:  filepath.Join(home, ".gemini"),
			Agent: "gemini",
			Alias: "gemini",
			Subdirs: map[string]string{
				source.KindContext: "",
			},
		},
		{
			Name:  "opencode",
			Path:  filepath.Join(home, ".config", "opencode"),
			Agent: "opencode",
			Alias: "opencode",
			Subdirs: map[string]string{
				source.KindSkill:   "skills",
				source.KindContext: "",
			},
		},
		{
			Name:  "agents",
			Path:  filepath.Join(home, ".agents"),
			Agent: "agents",
			Alias: "agents",
			Subdirs: map[string]string{
				source.KindSkill:   "skills",
				source.KindContext: "",
			},
		},
	}
}

// Discover returns the subset of KnownAgents whose Path exists on disk.
func Discover() []config.Target {
	var out []config.Target
	for _, a := range KnownAgents() {
		fi, err := os.Stat(a.Path)
		if err != nil || !fi.IsDir() {
			continue
		}
		out = append(out, a)
	}
	return out
}

// TargetFor constructs a Target for the given agent type, path, and name.
// Alias is set to agentName so targets of the same agent type can be grouped.
// Subdirs is left nil so SubdirFor and SupportsKind derive defaults from the
// agent profile. Callers can override fields after construction if needed.
func TargetFor(agentName, path, targetName string) config.Target {
	return config.Target{
		Name:  targetName,
		Path:  path,
		Agent: agentName,
		Alias: agentName,
	}
}
