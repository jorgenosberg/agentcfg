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
)

var profiles = map[string]Profile{
	Claude: {
		Subdirs:        map[string]string{source.KindSkill: "skills", source.KindHook: "hooks", source.KindContext: "", source.KindCommand: "commands"},
		SupportedKinds: []string{source.KindSkill, source.KindHook, source.KindContext, source.KindCommand},
	},
	Codex: {
		Subdirs:        map[string]string{source.KindSkill: "skills", source.KindContext: ""},
		SupportedKinds: []string{source.KindSkill, source.KindContext},
	},
	Copilot: {
		Subdirs:        map[string]string{source.KindContext: "", source.KindCommand: ".github/prompts"},
		SupportedKinds: []string{source.KindContext, source.KindCommand},
	},
	Gemini: {
		Subdirs:        map[string]string{source.KindContext: ""},
		SupportedKinds: []string{source.KindContext},
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
	},
	Opencode: {
		Subdirs:        map[string]string{source.KindSkill: "skills", source.KindContext: ""},
		SupportedKinds: []string{source.KindSkill, source.KindContext},
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
