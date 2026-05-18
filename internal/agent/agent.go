package agent

import "sort"

// Profile describes the default configuration for a known agent type.
type Profile struct {
	Subdirs        map[string]string // default per-kind subdirectory names
	SupportedKinds []string          // item kinds this agent supports
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
		Subdirs:        map[string]string{"skill": "skills", "hook": "hooks", "context": "", "command": "commands"},
		SupportedKinds: []string{"skill", "hook", "context", "command"},
	},
	Codex: {
		Subdirs:        map[string]string{"skill": "skills", "context": ""},
		SupportedKinds: []string{"skill", "context"},
	},
	Copilot: {
		Subdirs:        map[string]string{"context": ""},
		SupportedKinds: []string{"context"},
	},
	Gemini: {
		Subdirs:        map[string]string{"context": ""},
		SupportedKinds: []string{"context"},
	},
	Cursor: {
		Subdirs:        map[string]string{"rule": ""},
		SupportedKinds: []string{"rule"},
	},
	Cline: {
		Subdirs:        map[string]string{"rule": ""},
		SupportedKinds: []string{"rule"},
	},
	Windsurf: {
		Subdirs:        map[string]string{"rule": ""},
		SupportedKinds: []string{"rule"},
	},
	Aider: {
		Subdirs:        map[string]string{"context": ""},
		SupportedKinds: []string{"context"},
	},
	Agents: {
		Subdirs:        map[string]string{"skill": "skills", "context": ""},
		SupportedKinds: []string{"skill", "context"},
	},
	Opencode: {
		Subdirs:        map[string]string{"skill": "skills", "context": ""},
		SupportedKinds: []string{"skill", "context"},
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
