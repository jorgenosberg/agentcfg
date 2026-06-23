package forks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// ForkFile is the on-disk schema for ~/.agentcfg/forks.json.
type ForkFile struct {
	Version int             `json:"version"`
	Forks   map[string]Fork `json:"forks"` // keyed by "plugin@marketplace"
}

// Fork records one plugin fork operation.
type Fork struct {
	ForkedAt       time.Time `json:"forked_at"`
	SourceVersion  string    `json:"source_version"`
	PluginDisabled bool      `json:"plugin_disabled"`
	Skills         []string  `json:"skills"`
	Hooks          []string  `json:"hooks"`
	Skipped        Skipped   `json:"skipped"`
}

// Skipped lists components that could not be file-forked.
type Skipped struct {
	MCPServers []string `json:"mcp_servers,omitempty"`
	LSPServers []string `json:"lsp_servers,omitempty"`
}

// Load reads the forks file at path. Returns an empty ForkFile if the file is missing.
func Load(path string) (*ForkFile, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &ForkFile{Version: 1, Forks: map[string]Fork{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var f ForkFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, err
	}
	if f.Forks == nil {
		f.Forks = map[string]Fork{}
	}
	return &f, nil
}

// Save writes f to path, creating parent directories as needed.
func Save(path string, f *ForkFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if f.Forks == nil {
		f.Forks = map[string]Fork{}
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(path, raw, 0o644)
}

// DefaultPath returns ~/.agentcfg/forks.json.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agentcfg", "forks.json"), nil
}

// IsForked reports whether pluginFullName was forked and the given skillName is
// among the skills that were copied.
func (f *ForkFile) IsForked(pluginFullName, skillName string) bool {
	if f == nil {
		return false
	}
	fork, ok := f.Forks[pluginFullName]
	if !ok {
		return false
	}
	for _, s := range fork.Skills {
		if s == skillName {
			return true
		}
	}
	return false
}

// PluginForked reports whether any component of the given plugin was forked.
func (f *ForkFile) PluginForked(pluginFullName string) bool {
	if f == nil {
		return false
	}
	_, ok := f.Forks[pluginFullName]
	return ok
}
