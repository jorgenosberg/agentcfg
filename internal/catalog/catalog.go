// Package catalog lists well-known AI agent install directories.
//
// agentcfg uses this to (a) auto-populate sensible targets in `init` and
// (b) power the `discover` command that browses existing configs.
package catalog

import (
	"os"
	"path/filepath"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/source"
)

// KnownAgents returns the built-in catalog of agent install layouts.
// Paths use the current user's home directory.
func KnownAgents() []config.Target {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	skillsHooksContext := map[string]string{
		source.KindSkill:   "skills",
		source.KindHook:    "hooks",
		source.KindContext: "",
	}

	return []config.Target{
		{
			Name:    "claude",
			Path:    filepath.Join(home, ".claude"),
			Subdirs: skillsHooksContext,
		},
		{
			Name:    "codex",
			Path:    filepath.Join(home, ".codex"),
			Subdirs: skillsHooksContext,
		},
		{
			Name:    "opencode",
			Path:    filepath.Join(home, ".config", "opencode"),
			Subdirs: skillsHooksContext,
		},
		{
			Name: "copilot",
			Path: filepath.Join(home, ".config", "github-copilot"),
			Subdirs: map[string]string{
				source.KindSkill:   "prompts",
				source.KindContext: "",
			},
		},
		{
			Name:    "agents",
			Path:    filepath.Join(home, ".agents"),
			Subdirs: skillsHooksContext,
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
