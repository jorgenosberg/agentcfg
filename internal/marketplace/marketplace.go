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
	return filepath.Join(home, ".claude", "plugins", "known_marketplaces.json"), nil
}

// DefaultInstalledPluginsPath returns ~/.claude/plugins/installed_plugins.json.
func DefaultInstalledPluginsPath() (string, error) {
	home, err := paths.Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "plugins", "installed_plugins.json"), nil
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
	raw, err := os.ReadFile(knownMarketplacesPath)
	if os.IsNotExist(err) {
		raw = []byte("{}")
	} else if err != nil {
		return fmt.Errorf("read known_marketplaces: %w", err)
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse known_marketplaces: %w", err)
	}

	if _, exists := doc[MarketplaceName]; exists {
		return nil
	}

	type mpSource struct {
		Source string `json:"source"`
		Path   string `json:"path"`
	}
	type mpEntry struct {
		Source          mpSource `json:"source"`
		InstallLocation string   `json:"installLocation"`
		LastUpdated     string   `json:"lastUpdated"`
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

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')

	if err := os.MkdirAll(filepath.Dir(knownMarketplacesPath), 0o755); err != nil {
		return err
	}
	return atomicWrite(knownMarketplacesPath, out)
}

// RegisterInstalled adds a user-scope installed entry for p under
// ForkFullName(p.Name) to installedPluginsPath, pointing at bundlePath.
// If the fork is already listed, it is replaced (idempotent).
func RegisterInstalled(installedPluginsPath string, p plugins.Plugin, bundlePath string, now time.Time) error {
	raw, err := os.ReadFile(installedPluginsPath)
	if os.IsNotExist(err) {
		raw = []byte(`{"version":2,"plugins":{}}`)
	} else if err != nil {
		return fmt.Errorf("read installed_plugins: %w", err)
	}

	// Parse into a generic map to preserve all existing fields verbatim.
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse installed_plugins: %w", err)
	}

	// Read existing plugins map.
	pluginsMap := map[string]json.RawMessage{}
	if existing, ok := doc["plugins"]; ok {
		if err := json.Unmarshal(existing, &pluginsMap); err != nil {
			return fmt.Errorf("parse plugins map: %w", err)
		}
	}

	version := p.Version
	if version == "" {
		version = "unknown"
	}
	ts := now.Format(time.RFC3339Nano)

	type installedEntry struct {
		Scope       string `json:"scope"`
		InstallPath string `json:"installPath"`
		Version     string `json:"version"`
		InstalledAt string `json:"installedAt"`
		LastUpdated string `json:"lastUpdated"`
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

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')

	if err := os.MkdirAll(filepath.Dir(installedPluginsPath), 0o755); err != nil {
		return err
	}
	return atomicWrite(installedPluginsPath, out)
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
	return atomicWrite(path, raw)
}

// atomicWrite writes data to path via a sibling temp file + rename.
func atomicWrite(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".mp-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	return os.Rename(tmpName, path)
}
