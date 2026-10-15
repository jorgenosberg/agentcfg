package agent

import (
	"sort"

	"github.com/jorgenosberg/agentcfg/internal/source"
)

// Profile describes the default configuration for a known agent type.
type Profile struct {
	Subdirs        map[string]string // default per-kind subdirectory names
	SupportedKinds []string          // item kinds this agent supports
	// DestName optionally overrides the install filename for a given Kind.
	// Useful for agents that require a fixed filename (e.g. cursor ".cursorrules").
	// When absent, the source item's own name is used.
	DestName map[string]string
	// ProjectSubdirs maps kind to the path relative to a repository root
	// where the agent reads project-level items. A missing kind means the
	// agent has no project-level location for it.
	ProjectSubdirs map[string]string
}

const (
	Claude   = "claude"
	Codex    = "codex"
	Copilot  = "copilot"
	Gemini   = "gemini"
	Cursor   = "cursor"
	Cline    = "cline"
	Windsurf = "windsurf"
	Aider    = "aider"
	Agents   = "agents"
	Opencode = "opencode"
	Pi       = "pi"
)

var profiles = map[string]Profile{
	Claude: {
		Subdirs:        map[string]string{source.KindSkill: "skills", source.KindHook: "hooks", source.KindContext: "", source.KindCommand: "commands"},
		SupportedKinds: []string{source.KindSkill, source.KindHook, source.KindContext, source.KindCommand},
		ProjectSubdirs: map[string]string{source.KindSkill: ".claude/skills", source.KindHook: ".claude/hooks", source.KindContext: "", source.KindCommand: ".claude/commands"},
	},
	Codex: {
		Subdirs:        map[string]string{source.KindSkill: "skills", source.KindHook: "hooks", source.KindContext: ""},
		SupportedKinds: []string{source.KindSkill, source.KindHook, source.KindContext},
		ProjectSubdirs: map[string]string{source.KindSkill: ".agents/skills", source.KindHook: ".codex/hooks", source.KindContext: ""},
	},
	Copilot: {
		Subdirs:        map[string]string{source.KindSkill: "skills", source.KindHook: "hooks", source.KindContext: "", source.KindCommand: ".github/prompts"},
		SupportedKinds: []string{source.KindSkill, source.KindHook, source.KindContext, source.KindCommand},
		ProjectSubdirs: map[string]string{source.KindSkill: ".github/skills", source.KindHook: ".github/hooks", source.KindContext: "", source.KindCommand: ".github/prompts"},
	},
	Gemini: {
		// Antigravity CLI (fka Gemini CLI) keeps user skills under antigravity-cli/
		Subdirs:        map[string]string{source.KindSkill: "antigravity-cli/skills", source.KindContext: ""},
		SupportedKinds: []string{source.KindSkill, source.KindContext},
		ProjectSubdirs: map[string]string{source.KindSkill: ".agents/skills", source.KindContext: ""},
	},
	Cursor: {
		// Uses the modern multi-file form: .cursor/rules/
		Subdirs:        map[string]string{source.KindRule: ".cursor/rules"},
		SupportedKinds: []string{source.KindRule},
	},
	Cline: {
		// Single-file format: DestName renames the installed file to .clinerules
		Subdirs:        map[string]string{source.KindRule: ""},
		SupportedKinds: []string{source.KindRule},
		DestName:       map[string]string{source.KindRule: ".clinerules"},
	},
	Windsurf: {
		// Single-file format: DestName renames the installed file to .windsurfrules
		Subdirs:        map[string]string{source.KindRule: ""},
		SupportedKinds: []string{source.KindRule},
		DestName:       map[string]string{source.KindRule: ".windsurfrules"},
	},
	Aider: {
		Subdirs:        map[string]string{source.KindContext: ""},
		SupportedKinds: []string{source.KindContext},
	},
	Agents: {
		Subdirs:        map[string]string{source.KindSkill: "skills", source.KindContext: ""},
		SupportedKinds: []string{source.KindSkill, source.KindContext},
		ProjectSubdirs: map[string]string{source.KindSkill: ".agents/skills", source.KindContext: ""},
	},
	Opencode: {
		Subdirs:        map[string]string{source.KindSkill: "skills", source.KindContext: "", source.KindExtension: "plugins"},
		SupportedKinds: []string{source.KindSkill, source.KindContext, source.KindExtension},
		ProjectSubdirs: map[string]string{source.KindSkill: ".opencode/skills", source.KindContext: "", source.KindExtension: ".opencode/plugins"},
	},
	Pi: {
		Subdirs:        map[string]string{source.KindSkill: "skills", source.KindCommand: "prompts", source.KindExtension: "extensions"},
		SupportedKinds: []string{source.KindSkill, source.KindCommand, source.KindExtension},
		ProjectSubdirs: map[string]string{source.KindSkill: ".pi/skills", source.KindCommand: ".pi/prompts", source.KindExtension: ".pi/extensions"},
	},
}

// Get returns the Profile for the named agent type.
// The second return value is false if the name is not in the registry.
func Get(name string) (Profile, bool) {
	p, ok := profiles[name]
	return p, ok
}

// Names returns all registered agent type names, sorted alphabetically.
func Names() []string {
	out := make([]string, 0, len(profiles))
	for k := range profiles {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
