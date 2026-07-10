// Package marketplace manages the agentcfg-owned Claude Code plugin marketplace
// at ~/.agentcfg/forks/. Forked plugins live there as intact bundles and are
// registered with Claude Code so they load without any manual /plugin commands.
package marketplace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jorgenosberg/agentcfg/internal/fsutil"
	"github.com/jorgenosberg/agentcfg/internal/paths"
	"github.com/jorgenosberg/agentcfg/internal/plugins"
)

// MarketplaceName is the stable identifier used in known_marketplaces.json and
// in forked plugin full names ("plugin@agentcfg-forks").
const MarketplaceName = "agentcfg-forks"

// DefaultForksRoot returns the path to the agentcfg fork marketplace root directory.
func DefaultForksRoot() (string, error) {
	home, err := paths.Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agentcfg", "forks"), nil
}

// DefaultKnownMarketplacesPath returns ~/.claude/plugins/known_marketplaces.json.
func DefaultKnownMarketplacesPath() (string, error) {
	home, err := paths.Home()
	if err != nil {
		return "", err
	}
	return KnownMarketplacesPathIn(filepath.Join(home, ".claude")), nil
}

// KnownMarketplacesPathIn returns known_marketplaces.json inside an arbitrary
// Claude Code directory.
func KnownMarketplacesPathIn(claudeDir string) string {
	return filepath.Join(claudeDir, "plugins", "known_marketplaces.json")
}

// DefaultInstalledPluginsPath returns ~/.claude/plugins/installed_plugins.json.
func DefaultInstalledPluginsPath() (string, error) {
	home, err := paths.Home()
	if err != nil {
		return "", err
	}
	return InstalledPluginsPathIn(filepath.Join(home, ".claude")), nil
}

// InstalledPluginsPathIn returns installed_plugins.json inside an arbitrary
// Claude Code directory.
func InstalledPluginsPathIn(claudeDir string) string {
	return filepath.Join(claudeDir, "plugins", "installed_plugins.json")
}

// ManifestPath returns the marketplace.json path inside forksRoot.
// The file lives at <forksRoot>/.claude-plugin/marketplace.json to match
// the layout Claude Code expects for local-directory marketplaces.
func ManifestPath(forksRoot string) string {
	return filepath.Join(forksRoot, ".claude-plugin", "marketplace.json")
}

// BundlePath returns the path where a plugin's bundle is stored inside forksRoot.
func BundlePath(forksRoot, pluginName string) string {
	return filepath.Join(forksRoot, "plugins", pluginName)
}

// ForkFullName returns the plugin's full identifier in the agentcfg-forks marketplace.
func ForkFullName(pluginName string) string {
	return pluginName + "@" + MarketplaceName
}

// manifest is the structure of the marketplace's .claude-plugin/marketplace.json.
type manifest struct {
	Schema      string           `json:"$schema,omitempty"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Plugins     []manifestPlugin `json:"plugins"`
}

type manifestPlugin struct {
	Name        string `json:"name"`
	Source      string `json:"source"`
	Description string `json:"description,omitempty"`
}

// AddPlugin adds p to the marketplace manifest inside forksRoot, creating it if
// needed. If the plugin is already listed, it is replaced (idempotent).
func AddPlugin(forksRoot string, p plugins.Plugin) error {
	mp := ManifestPath(forksRoot)
	if err := os.MkdirAll(filepath.Dir(mp), 0o755); err != nil {
		return fmt.Errorf("create manifest dir: %w", err)
	}

	m, err := loadManifest(mp)
	if err != nil {
		return fmt.Errorf("load marketplace manifest: %w", err)
	}

	// Rebuild, replacing any existing entry for this plugin.
	out := make([]manifestPlugin, 0, len(m.Plugins)+1)
	for _, ep := range m.Plugins {
		if ep.Name != p.Name {
			out = append(out, ep)
		}
	}
	out = append(out, manifestPlugin{
		Name:        p.Name,
		Source:      "./plugins/" + p.Name,
		Description: "Forked from " + p.FullName,
	})
	m.Plugins = out

	return saveManifest(mp, m)
}

// RegisterMarketplace adds agentcfg-forks to knownMarketplacesPath if absent.
// The entry points at forksRoot as a local directory source. Idempotent.
func RegisterMarketplace(knownMarketplacesPath, forksRoot string) error {
	type mpSource struct {
		Source string `json:"source"`
		Path   string `json:"path"`
	}
	type mpEntry struct {
		Source          mpSource `json:"source"`
		InstallLocation string   `json:"installLocation"`
		LastUpdated     string   `json:"lastUpdated"`
	}
	return fsutil.EditJSON(knownMarketplacesPath, []byte("{}"), func(doc map[string]json.RawMessage) error {
		if _, exists := doc[MarketplaceName]; exists {
			return nil
		}
		entry := mpEntry{
			Source:          mpSource{Source: "directory", Path: forksRoot},
			InstallLocation: forksRoot,
			LastUpdated:     time.Now().UTC().Format(time.RFC3339Nano),
		}
		entryRaw, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		doc[MarketplaceName] = json.RawMessage(entryRaw)
		return nil
	})
}

// RegisterInstalled adds a user-scope installed entry for p under
// ForkFullName(p.Name) to installedPluginsPath, pointing at bundlePath.
// If the fork is already listed, it is replaced (idempotent).
func RegisterInstalled(installedPluginsPath string, p plugins.Plugin, bundlePath string, now time.Time) error {
	type installedEntry struct {
		Scope       string `json:"scope"`
		InstallPath string `json:"installPath"`
		Version     string `json:"version"`
		InstalledAt string `json:"installedAt"`
		LastUpdated string `json:"lastUpdated"`
	}
	version := p.Version
	if version == "" {
		version = "unknown"
	}
	ts := now.Format(time.RFC3339Nano)

	return fsutil.EditJSON(installedPluginsPath, []byte(`{"version":2,"plugins":{}}`), func(doc map[string]json.RawMessage) error {
		pluginsMap := map[string]json.RawMessage{}
		if existing, ok := doc["plugins"]; ok {
			if err := json.Unmarshal(existing, &pluginsMap); err != nil {
				return fmt.Errorf("parse plugins map: %w", err)
			}
		}
		entry := []installedEntry{{
			Scope:       "user",
			InstallPath: bundlePath,
			Version:     version,
			InstalledAt: ts,
			LastUpdated: ts,
		}}
		entryRaw, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		pluginsMap[ForkFullName(p.Name)] = json.RawMessage(entryRaw)
		pluginsRaw, err := json.Marshal(pluginsMap)
		if err != nil {
			return err
		}
		doc["plugins"] = json.RawMessage(pluginsRaw)
		if _, ok := doc["version"]; !ok {
			doc["version"] = json.RawMessage(`2`)
		}
		return nil
	})
}

// — manifest helpers —

func loadManifest(path string) (manifest, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return manifest{
			Schema:      "https://anthropic.com/claude-code/marketplace.schema.json",
			Name:        MarketplaceName,
			Description: "Plugins forked and owned via agentcfg",
			Plugins:     nil,
		}, nil
	}
	if err != nil {
		return manifest{}, err
	}
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return manifest{}, err
	}
	return m, nil
}

func saveManifest(path string, m manifest) error {
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return fsutil.AtomicWrite(path, raw)
}
