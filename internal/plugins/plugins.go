package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jorgenosberg/agentcfg/internal/paths"
)

// Plugin represents one Claude Code plugin and its current state.
type Plugin struct {
	Name         string
	Marketplace  string
	FullName     string // "name@marketplace"
	Installed    bool
	Enabled      bool
	InstallPath  string // ~/.claude/plugins/cache/marketplace/plugin/version
	Version      string
	GitCommitSha string
	Skills       []string
	Hooks        []string
	MCPServers   []string
	LSPServers   []string
}

// Registry holds all known Claude Code plugins joined from the three state files.
type Registry struct {
	Plugins []Plugin
}

// DefaultDir returns the Claude Code plugins directory.
func DefaultDir() (string, error) {
	home, err := paths.Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "plugins"), nil
}

// Load reads Claude Code's three plugin state files and returns a Registry.
// If the plugin directory does not exist, an empty Registry is returned without error.
func Load() (*Registry, error) {
	dir, err := DefaultDir()
	if err != nil {
		return &Registry{}, nil
	}
	return LoadFrom(dir)
}

// LoadFrom is Load for an arbitrary Claude Code plugins directory
// (typically "<claude-dir>/plugins"), letting callers inspect a specific
// Claude Code profile instead of the default ~/.claude one.
func LoadFrom(dir string) (*Registry, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return &Registry{}, nil
	}

	settingsPath := filepath.Join(filepath.Dir(dir), "settings.json")
	installedPath := filepath.Join(dir, "installed_plugins.json")
	catalogPath := filepath.Join(dir, "plugin-catalog-cache.json")

	enabled, err := loadEnabledPlugins(settingsPath)
	if err != nil {
		return &Registry{}, fmt.Errorf("load plugin settings: %w", err)
	}

	installed, err := loadInstalled(installedPath)
	if err != nil {
		return &Registry{}, fmt.Errorf("load installed plugins: %w", err)
	}

	components, err := loadCatalog(catalogPath)
	if err != nil {
		return &Registry{}, fmt.Errorf("load plugin catalog: %w", err)
	}

	// Merge: start from installed set, overlay catalog components and enabled state.
	pluginMap := make(map[string]*Plugin, len(installed))
	for fullName, inst := range installed {
		pluginMap[fullName] = inst
	}
	for fullName, comp := range components {
		p, ok := pluginMap[fullName]
		if !ok {
			// Known in catalog but not installed — skip.
			continue
		}
		p.Skills = comp.skills
		p.Hooks = comp.hooks
		p.MCPServers = comp.mcpServers
		p.LSPServers = comp.lspServers
	}
	for fullName, p := range pluginMap {
		en, inMap := enabled[fullName]
		// If absent from enabledPlugins map, default to true (Claude Code default).
		p.Enabled = !inMap || en
		_ = inMap
	}

	out := make([]Plugin, 0, len(pluginMap))
	for _, p := range pluginMap {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FullName < out[j].FullName })

	return &Registry{Plugins: out}, nil
}

// Get returns the Plugin with the given full name (e.g. "name@marketplace"), or false if not found.
func (r *Registry) Get(fullName string) (Plugin, bool) {
	for _, p := range r.Plugins {
		if p.FullName == fullName {
			return p, true
		}
	}
	return Plugin{}, false
}

// — file parsers —

// installedEntry mirrors one element of the installed_plugins.json plugins array.
type installedEntry struct {
	Scope        string `json:"scope"`
	ProjectPath  string `json:"projectPath"`
	InstallPath  string `json:"installPath"`
	Version      string `json:"version"`
	GitCommitSha string `json:"gitCommitSha"`
}

type installedFile struct {
	Version int                         `json:"version"`
	Plugins map[string][]installedEntry `json:"plugins"`
}

func loadInstalled(path string) (map[string]*Plugin, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]*Plugin{}, nil
	}
	if err != nil {
		return nil, err
	}
	var f installedFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, err
	}

	out := make(map[string]*Plugin, len(f.Plugins))
	for fullName, entries := range f.Plugins {
		if len(entries) == 0 {
			continue
		}
		// Prefer user scope; fall back to first entry.
		chosen := entries[0]
		for _, e := range entries {
			if e.Scope == "user" {
				chosen = e
				break
			}
		}
		name, marketplace := splitFullName(fullName)
		out[fullName] = &Plugin{
			Name:         name,
			Marketplace:  marketplace,
			FullName:     fullName,
			Installed:    true,
			InstallPath:  chosen.InstallPath,
			Version:      chosen.Version,
			GitCommitSha: chosen.GitCommitSha,
		}
	}
	return out, nil
}

type catalogComponents struct {
	skills     []string
	hooks      []string
	mcpServers []string
	lspServers []string
}

type catalogPlugin struct {
	Components struct {
		// Skills and Hooks are arrays of {"name": ..., ...} objects in the
		// real catalog cache; MCPServers and LSPServers are arrays of plain
		// server-name strings.
		Skills []struct {
			Name string `json:"name"`
		} `json:"skills"`
		Hooks []struct {
			Name string `json:"name"`
		} `json:"hooks"`
		MCPServers []string `json:"mcpServers"`
		LSPServers []string `json:"lspServers"`
	} `json:"components"`
}

type catalogFile struct {
	Catalog struct {
		Plugins map[string]catalogPlugin `json:"plugins"`
	} `json:"catalog"`
}

func loadCatalog(path string) (map[string]catalogComponents, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]catalogComponents{}, nil
	}
	if err != nil {
		return nil, err
	}
	var f catalogFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, err
	}

	out := make(map[string]catalogComponents, len(f.Catalog.Plugins))
	for fullName, p := range f.Catalog.Plugins {
		var comp catalogComponents
		for _, s := range p.Components.Skills {
			comp.skills = append(comp.skills, s.Name)
		}
		for _, h := range p.Components.Hooks {
			comp.hooks = append(comp.hooks, h.Name)
		}
		comp.mcpServers = append(comp.mcpServers, p.Components.MCPServers...)
		comp.lspServers = append(comp.lspServers, p.Components.LSPServers...)
		out[fullName] = comp
	}
	return out, nil
}

type settingsPartial struct {
	EnabledPlugins map[string]bool `json:"enabledPlugins"`
}

func loadEnabledPlugins(path string) (map[string]bool, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	var s settingsPartial
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	if s.EnabledPlugins == nil {
		return map[string]bool{}, nil
	}
	return s.EnabledPlugins, nil
}

// splitFullName splits "name@marketplace" into its two parts.
// If the string has no "@", the whole string is the name and marketplace is "".
func splitFullName(full string) (name, marketplace string) {
	for i := len(full) - 1; i >= 0; i-- {
		if full[i] == '@' {
			return full[:i], full[i+1:]
		}
	}
	return full, ""
}
