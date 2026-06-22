package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jorgenosberg/agentcfg/internal/agent"
	"github.com/jorgenosberg/agentcfg/internal/source"
)

const (
	StrategyLink = "link"
	StrategyCopy = "copy"
)

// Config is the on-disk schema for ~/.agentcfg/config.json.
//
// Strategy values:
//   - "link" — symlinks installed entries to the source. Fast, single
//     source of truth; some agents may not resolve symlinks correctly.
//   - "copy" — snapshots source into the target. Safe with any agent but
//     requires re-sync after source edits.
type Config struct {
	// Source is the root directory holding skills/, hooks/, context/.
	Source string `json:"source"`
	// DefaultStrategy is applied to targets that do not override it.
	DefaultStrategy string `json:"default_strategy"`
	// Projects are repository/workspace folders scanned for in-repo agent
	// configuration files (CLAUDE.md, .github/copilot-instructions.md, etc.).
	// These are discovery-only and are not synced or installed by agentcfg.
	Projects []Project `json:"projects,omitempty"`
	// Targets are the agent directories to sync into.
	Targets []Target `json:"targets"`
}

// Project is a repository or workspace folder to scan for in-repo agent
// configuration files such as CLAUDE.md, .github/copilot-instructions.md,
// .claude/skills/, .cursorrules, and similar per-agent artefacts.
type Project struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Target is one AI agent install destination.
type Target struct {
	Name     string            `json:"name"`
	Path     string            `json:"path"`
	Agent    string            `json:"agent,omitempty"`    // agent type: "claude", "codex", etc.
	Alias    string            `json:"alias,omitempty"`    // optional group name; multiple targets may share one alias
	Strategy string            `json:"strategy,omitempty"` // overrides Config.DefaultStrategy
	Subdirs  map[string]string `json:"subdirs,omitempty"`  // per-kind subdir overrides
	Exclude  []string          `json:"exclude,omitempty"`  // "kind/name" pairs to skip, e.g. "context/GEMINI.md"
	Disabled []string          `json:"disabled,omitempty"` // user-toggled-off item names
}

// Excludes reports whether it should be skipped for this target.
func (t Target) Excludes(it source.Item) bool {
	for _, e := range t.Exclude {
		if e == it.Kind+"/"+it.Name || e == it.Name {
			return true
		}
	}
	return false
}

// IsDisabled reports whether the user has toggled this item off for this target.
// It checks both "kind/name" and plain "name" entries, using the same format as
// Exclude entries. Unlike Exclude, Disabled is a reversible user preference.
func (t Target) IsDisabled(it source.Item) bool {
	for _, d := range t.Disabled {
		if d == it.Kind+"/"+it.Name || d == it.Name {
			return true
		}
	}
	return false
}

// ResolveStrategy returns Target.Strategy if set, else fallback. Callers
// pass Config.DefaultStrategy as fallback; this avoids mutating Target on
// Load (which would persist back to disk on Save).
func (t Target) ResolveStrategy(fallback string) string {
	if t.Strategy != "" {
		return t.Strategy
	}
	if fallback != "" {
		return fallback
	}
	return StrategyLink
}

// SupportsKind reports whether this target supports the given item kind.
// Precedence: explicit Subdirs override agent profile. If neither is set,
// all kinds are supported.
func (t Target) SupportsKind(kind string) bool {
	if t.Subdirs != nil {
		_, ok := t.Subdirs[kind]
		return ok
	}
	if t.Agent != "" {
		if p, ok := agent.Get(t.Agent); ok {
			for _, k := range p.SupportedKinds {
				if k == kind {
					return true
				}
			}
			return false
		}
	}
	return true
}

// SubdirFor returns the per-kind subdirectory under the target root.
// When Subdirs is explicitly set, it is the complete specification and there
// is no fallthrough to the agent profile or defaultSubdirs. Otherwise,
// precedence is: agent profile → defaultSubdirs.
func (t Target) SubdirFor(kind string) string {
	if t.Subdirs != nil {
		return t.Subdirs[kind]
	}
	if t.Agent != "" {
		if p, ok := agent.Get(t.Agent); ok {
			if v, ok := p.Subdirs[kind]; ok {
				return v
			}
		}
	}
	return defaultSubdirs[kind]
}

var defaultSubdirs = map[string]string{
	source.KindSkill:   "skills",
	source.KindHook:    "hooks",
	source.KindContext: "",
}

// DefaultPath returns the canonical config file path.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agentcfg", "config.json"), nil
}

// DefaultSource returns the agentcfg-owned source directory. Users can
// populate it directly or symlink it to a path of their choice.
func DefaultSource() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agentcfg", "source"), nil
}

// Default returns the seed configuration written by `agentcfg init`.
// source overrides DefaultSource() when non-empty. Targets is left empty;
// users add them via `agentcfg target add` or by editing the config file.
func Default(source string) Config {
	if source == "" {
		s, _ := DefaultSource()
		source = s
	}
	return Config{
		Source:          source,
		DefaultStrategy: StrategyLink,
		Projects:        []Project{},
		Targets:         []Target{},
	}
}

// Load reads and validates the config at path. If the file does not exist,
// Default("") is returned without error so first-run still works.
func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Default(""), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if c.Projects == nil {
		c.Projects = []Project{}
	}
	if c.Targets == nil {
		c.Targets = []Target{}
	}
	if c.DefaultStrategy == "" {
		c.DefaultStrategy = StrategyLink
	}
	return c, nil
}

// Save writes the config to path as indented JSON, creating parent dirs.
func Save(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if c.Projects == nil {
		c.Projects = []Project{}
	}
	if c.Targets == nil {
		c.Targets = []Target{}
	}
	body, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o644)
}
