package forks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/jorgenosberg/agentcfg/internal/paths"
)

// ForkFile is the on-disk schema for ~/.agentcfg/forks.json.
type ForkFile struct {
	Version int             `json:"version"`
	Forks   map[string]Fork `json:"forks"` // keyed by upstream "plugin@marketplace"
}

// Fork records one plugin fork operation.
type Fork struct {
	ForkedAt         time.Time `json:"forked_at"`
	SourceVersion    string    `json:"source_version"`    // upstream GitCommitSha at fork time
	UpstreamFullName string    `json:"upstream_full_name"` // "<name>@<marketplace>"
	ForkFullName     string    `json:"fork_full_name"`     // "<name>@agentcfg-forks"
	BundlePath       string    `json:"bundle_path"`        // ~/.agentcfg/forks/plugins/<name>
	UpstreamDisabled bool      `json:"upstream_disabled"`
}

// Load reads the forks file at path. Returns an empty ForkFile if the file is missing.
func Load(path string) (*ForkFile, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &ForkFile{Version: 2, Forks: map[string]Fork{}}, nil
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

// DefaultPath returns the agentcfg forks file path.
func DefaultPath() (string, error) {
	home, err := paths.Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agentcfg", "forks.json"), nil
}

// PluginForked reports whether the given upstream plugin full name was forked.
func (f *ForkFile) PluginForked(upstreamFullName string) bool {
	if f == nil {
		return false
	}
	_, ok := f.Forks[upstreamFullName]
	return ok
}
