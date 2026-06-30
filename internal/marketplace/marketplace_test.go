package marketplace_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jorgenosberg/agentcfg/internal/marketplace"
	"github.com/jorgenosberg/agentcfg/internal/plugins"
)

func testPlugin() plugins.Plugin {
	return plugins.Plugin{
		Name:        "myplugin",
		Marketplace: "mymarket",
		FullName:    "myplugin@mymarket",
		Version:     "1.0.0",
	}
}

func TestAddPlugin_CreatesManifest(t *testing.T) {
	forksRoot := t.TempDir()
	p := testPlugin()

	if err := marketplace.AddPlugin(forksRoot, p); err != nil {
		t.Fatalf("AddPlugin: %v", err)
	}

	mp := marketplace.ManifestPath(forksRoot)
	raw, err := os.ReadFile(mp)
	if err != nil {
		t.Fatalf("marketplace.json not created: %v", err)
	}

	var m struct {
		Name    string `json:"name"`
		Plugins []struct {
			Name   string `json:"name"`
			Source string `json:"source"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if m.Name != marketplace.MarketplaceName {
		t.Errorf("Name: got %q want %q", m.Name, marketplace.MarketplaceName)
	}
	if len(m.Plugins) != 1 || m.Plugins[0].Name != "myplugin" {
		t.Errorf("Plugins: %+v", m.Plugins)
	}
	if m.Plugins[0].Source != "./plugins/myplugin" {
		t.Errorf("Source: got %q", m.Plugins[0].Source)
	}
}

func TestAddPlugin_Idempotent(t *testing.T) {
	forksRoot := t.TempDir()
	p := testPlugin()

	// Add twice; should not duplicate.
	if err := marketplace.AddPlugin(forksRoot, p); err != nil {
		t.Fatalf("first AddPlugin: %v", err)
	}
	if err := marketplace.AddPlugin(forksRoot, p); err != nil {
		t.Fatalf("second AddPlugin: %v", err)
	}

	raw, _ := os.ReadFile(marketplace.ManifestPath(forksRoot))
	var m struct {
		Plugins []struct{ Name string } `json:"plugins"`
	}
	json.Unmarshal(raw, &m)
	if len(m.Plugins) != 1 {
		t.Errorf("expected 1 plugin entry after duplicate AddPlugin, got %d", len(m.Plugins))
	}
}

func TestAddPlugin_PreservesOtherPlugins(t *testing.T) {
	forksRoot := t.TempDir()

	// Add first plugin.
	p1 := plugins.Plugin{Name: "alpha", FullName: "alpha@mp"}
	if err := marketplace.AddPlugin(forksRoot, p1); err != nil {
		t.Fatal(err)
	}

	// Add second plugin.
	p2 := plugins.Plugin{Name: "beta", FullName: "beta@mp"}
	if err := marketplace.AddPlugin(forksRoot, p2); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(marketplace.ManifestPath(forksRoot))
	var m struct {
		Plugins []struct{ Name string } `json:"plugins"`
	}
	json.Unmarshal(raw, &m)
	if len(m.Plugins) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(m.Plugins))
	}
}

func TestRegisterMarketplace_CreatesEntry(t *testing.T) {
	tmp := t.TempDir()
	knownMPPath := filepath.Join(tmp, "known_marketplaces.json")
	forksRoot := filepath.Join(tmp, "forks")

	if err := marketplace.RegisterMarketplace(knownMPPath, forksRoot); err != nil {
		t.Fatalf("RegisterMarketplace: %v", err)
	}

	raw, err := os.ReadFile(knownMPPath)
	if err != nil {
		t.Fatalf("known_marketplaces.json not created: %v", err)
	}

	var doc map[string]struct {
		Source struct {
			Source string `json:"source"`
			Path   string `json:"path"`
		} `json:"source"`
		InstallLocation string `json:"installLocation"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	entry, ok := doc[marketplace.MarketplaceName]
	if !ok {
		t.Fatalf("agentcfg-forks not in known_marketplaces.json")
	}
	if entry.Source.Source != "directory" {
		t.Errorf("source.source: got %q want directory", entry.Source.Source)
	}
	if entry.Source.Path != forksRoot {
		t.Errorf("source.path: got %q want %q", entry.Source.Path, forksRoot)
	}
	if entry.InstallLocation != forksRoot {
		t.Errorf("installLocation: got %q want %q", entry.InstallLocation, forksRoot)
	}
}

func TestRegisterMarketplace_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	knownMPPath := filepath.Join(tmp, "known_marketplaces.json")
	forksRoot := filepath.Join(tmp, "forks")

	if err := marketplace.RegisterMarketplace(knownMPPath, forksRoot); err != nil {
		t.Fatal(err)
	}
	if err := marketplace.RegisterMarketplace(knownMPPath, forksRoot); err != nil {
		t.Fatalf("second call: %v", err)
	}

	raw, _ := os.ReadFile(knownMPPath)
	var doc map[string]json.RawMessage
	json.Unmarshal(raw, &doc)
	if len(doc) != 1 {
		t.Errorf("expected 1 entry after duplicate register, got %d", len(doc))
	}
}

func TestRegisterMarketplace_PreservesExistingEntries(t *testing.T) {
	tmp := t.TempDir()
	knownMPPath := filepath.Join(tmp, "known_marketplaces.json")

	// Seed an existing marketplace entry.
	os.WriteFile(knownMPPath, []byte(`{"some-other":{"source":{"source":"github","repo":"foo/bar"},"installLocation":"/tmp/foo","lastUpdated":"2026-01-01T00:00:00Z"}}`), 0o644)

	if err := marketplace.RegisterMarketplace(knownMPPath, "/tmp/forks"); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(knownMPPath)
	var doc map[string]json.RawMessage
	json.Unmarshal(raw, &doc)
	if _, ok := doc["some-other"]; !ok {
		t.Error("existing entry 'some-other' was removed")
	}
	if _, ok := doc[marketplace.MarketplaceName]; !ok {
		t.Error("agentcfg-forks was not added")
	}
}

func TestRegisterInstalled_CreatesEntry(t *testing.T) {
	tmp := t.TempDir()
	installedPath := filepath.Join(tmp, "installed_plugins.json")
	bundlePath := filepath.Join(tmp, "forks", "plugins", "myplugin")
	p := testPlugin()
	now := time.Now().UTC()

	if err := marketplace.RegisterInstalled(installedPath, p, bundlePath, now); err != nil {
		t.Fatalf("RegisterInstalled: %v", err)
	}

	raw, err := os.ReadFile(installedPath)
	if err != nil {
		t.Fatalf("installed_plugins.json not created: %v", err)
	}

	var doc struct {
		Plugins map[string][]struct {
			Scope       string `json:"scope"`
			InstallPath string `json:"installPath"`
			Version     string `json:"version"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	forkKey := marketplace.ForkFullName(p.Name)
	entries, ok := doc.Plugins[forkKey]
	if !ok {
		t.Fatalf("%q not in installed_plugins.json", forkKey)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Scope != "user" {
		t.Errorf("scope: got %q want user", entries[0].Scope)
	}
	if entries[0].InstallPath != bundlePath {
		t.Errorf("installPath: got %q want %q", entries[0].InstallPath, bundlePath)
	}
	if entries[0].Version != "1.0.0" {
		t.Errorf("version: got %q", entries[0].Version)
	}
}

func TestRegisterInstalled_PreservesExistingEntries(t *testing.T) {
	tmp := t.TempDir()
	installedPath := filepath.Join(tmp, "installed_plugins.json")

	// Seed with an existing plugin entry.
	existing := map[string]any{
		"version": 2,
		"plugins": map[string]any{
			"other-plugin@market": []map[string]any{{
				"scope":       "user",
				"installPath": "/somewhere/other",
				"version":     "2.0.0",
			}},
		},
	}
	raw, _ := json.MarshalIndent(existing, "", "  ")
	os.WriteFile(installedPath, append(raw, '\n'), 0o644)

	p := testPlugin()
	if err := marketplace.RegisterInstalled(installedPath, p, "/tmp/bundle", time.Now()); err != nil {
		t.Fatal(err)
	}

	raw, _ = os.ReadFile(installedPath)
	var doc struct {
		Plugins map[string]json.RawMessage `json:"plugins"`
	}
	json.Unmarshal(raw, &doc)
	if _, ok := doc.Plugins["other-plugin@market"]; !ok {
		t.Error("existing entry 'other-plugin@market' was removed")
	}
	if _, ok := doc.Plugins[marketplace.ForkFullName(p.Name)]; !ok {
		t.Error("fork entry was not added")
	}
}

func TestBundlePath(t *testing.T) {
	got := marketplace.BundlePath("/root/forks", "myplugin")
	want := "/root/forks/plugins/myplugin"
	if got != want {
		t.Errorf("BundlePath: got %q want %q", got, want)
	}
}

func TestForkFullName(t *testing.T) {
	got := marketplace.ForkFullName("myplugin")
	if got != "myplugin@agentcfg-forks" {
		t.Errorf("ForkFullName: got %q", got)
	}
}
