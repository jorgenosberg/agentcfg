package fork_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jorgenosberg/agentcfg/internal/fork"
	"github.com/jorgenosberg/agentcfg/internal/forks"
	"github.com/jorgenosberg/agentcfg/internal/marketplace"
	"github.com/jorgenosberg/agentcfg/internal/plugins"
)

// makePlugin builds a realistic plugin directory structure and returns the Plugin.
// The bundle has: skills/, commands/, agents/, hooks/, bin/ — testing that the
// full bundle is copied, not just skills/hooks.
func makePlugin(t *testing.T, installDir string) plugins.Plugin {
	t.Helper()

	// skills/my-skill/SKILL.md
	mkfile(t, filepath.Join(installDir, "skills", "my-skill", "SKILL.md"), "---\nname: my-skill\n---\nContent.")
	// commands/review.md
	mkfile(t, filepath.Join(installDir, "commands", "review.md"), "# review command")
	// agents/helper.md
	mkfile(t, filepath.Join(installDir, "agents", "helper.md"), "# agent")
	// hooks/on-start.sh
	mkfile(t, filepath.Join(installDir, "hooks", "on-start.sh"), "#!/bin/sh\necho hook")
	// bin/tool
	mkfile(t, filepath.Join(installDir, "bin", "tool"), "#!/bin/sh\necho tool")
	// .claude-plugin/plugin.json
	mkfile(t, filepath.Join(installDir, ".claude-plugin", "plugin.json"),
		`{"name":"testplugin","version":"1.0.0"}`)

	return plugins.Plugin{
		Name:        "testplugin",
		Marketplace: "testmarket",
		FullName:    "testplugin@testmarket",
		Installed:   true,
		Enabled:     true,
		InstallPath: installDir,
		Version:     "1.0.0",
		GitCommitSha: "deadbeef1234",
		Skills:      []string{"my-skill"},
		Hooks:       []string{"on-start.sh"},
		MCPServers:  []string{"mcp-server"},
	}
}

func mkfile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdirall %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("mkfile %s: %v", path, err)
	}
}

func buildRequest(t *testing.T, tmp string, p plugins.Plugin) fork.Request {
	t.Helper()
	forksRoot := filepath.Join(tmp, "agentcfg-forks")
	forksPath := filepath.Join(tmp, "forks.json")
	settingsPath := filepath.Join(tmp, "settings.json")
	knownMPPath := filepath.Join(tmp, "known_marketplaces.json")
	installedPath := filepath.Join(tmp, "installed_plugins.json")

	// Seed settings.json with upstream enabled.
	mkfile(t, settingsPath, `{"enabledPlugins":{"testplugin@testmarket":true}}`)

	return fork.Request{
		Plugin:                p,
		ForksRoot:             forksRoot,
		ForksPath:             forksPath,
		SettingsPath:          settingsPath,
		KnownMarketplacesPath: knownMPPath,
		InstalledPluginsPath:  installedPath,
	}
}

func TestExecute_BundleCopiedFully(t *testing.T) {
	tmp := t.TempDir()
	installDir := filepath.Join(tmp, "cache", "testplugin", "1.0.0")
	p := makePlugin(t, installDir)
	req := buildRequest(t, tmp, p)

	res, err := fork.Execute(req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	bundleDest := marketplace.BundlePath(req.ForksRoot, p.Name)
	if res.BundlePath != bundleDest {
		t.Errorf("BundlePath: got %q want %q", res.BundlePath, bundleDest)
	}
	if res.ForkFullName != "testplugin@agentcfg-forks" {
		t.Errorf("ForkFullName: got %q", res.ForkFullName)
	}

	// Entire bundle should be present — including non-skill/hook dirs.
	for _, rel := range []string{
		"skills/my-skill/SKILL.md",
		"commands/review.md",
		"agents/helper.md",
		"hooks/on-start.sh",
		"bin/tool",
		".claude-plugin/plugin.json",
	} {
		path := filepath.Join(bundleDest, rel)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s in bundle, not found: %v", rel, err)
		}
	}
}

func TestExecute_MarketplaceManifestWritten(t *testing.T) {
	tmp := t.TempDir()
	p := makePlugin(t, filepath.Join(tmp, "cache", "testplugin", "1.0.0"))
	req := buildRequest(t, tmp, p)

	if _, err := fork.Execute(req); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// marketplace.json must exist inside the forks root
	mp := filepath.Join(req.ForksRoot, ".claude-plugin", "marketplace.json")
	raw, err := os.ReadFile(mp)
	if err != nil {
		t.Fatalf("marketplace.json not created: %v", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("marketplace.json invalid JSON: %v", err)
	}
}

func TestExecute_KnownMarketplacesRegistered(t *testing.T) {
	tmp := t.TempDir()
	p := makePlugin(t, filepath.Join(tmp, "cache", "testplugin", "1.0.0"))
	req := buildRequest(t, tmp, p)

	if _, err := fork.Execute(req); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	raw, err := os.ReadFile(req.KnownMarketplacesPath)
	if err != nil {
		t.Fatalf("known_marketplaces.json not created: %v", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("known_marketplaces.json invalid JSON: %v", err)
	}
	if _, ok := doc[marketplace.MarketplaceName]; !ok {
		t.Errorf("agentcfg-forks not registered in known_marketplaces.json")
	}
}

func TestExecute_InstalledPluginsRegistered(t *testing.T) {
	tmp := t.TempDir()
	p := makePlugin(t, filepath.Join(tmp, "cache", "testplugin", "1.0.0"))
	req := buildRequest(t, tmp, p)

	if _, err := fork.Execute(req); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	raw, err := os.ReadFile(req.InstalledPluginsPath)
	if err != nil {
		t.Fatalf("installed_plugins.json not created: %v", err)
	}
	var doc struct {
		Plugins map[string]json.RawMessage `json:"plugins"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("installed_plugins.json invalid JSON: %v", err)
	}
	if _, ok := doc.Plugins["testplugin@agentcfg-forks"]; !ok {
		t.Error("testplugin@agentcfg-forks not in installed_plugins.json")
	}
}

func TestExecute_UpstreamDisabledForkEnabled(t *testing.T) {
	tmp := t.TempDir()
	p := makePlugin(t, filepath.Join(tmp, "cache", "testplugin", "1.0.0"))
	req := buildRequest(t, tmp, p)

	if _, err := fork.Execute(req); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	raw, _ := os.ReadFile(req.SettingsPath)
	var settings struct {
		EnabledPlugins map[string]bool `json:"enabledPlugins"`
	}
	json.Unmarshal(raw, &settings)

	if settings.EnabledPlugins["testplugin@testmarket"] {
		t.Error("upstream should be disabled in settings.json")
	}
	if !settings.EnabledPlugins["testplugin@agentcfg-forks"] {
		t.Error("fork should be enabled in settings.json")
	}
}

func TestExecute_ForksJsonUpdated(t *testing.T) {
	tmp := t.TempDir()
	p := makePlugin(t, filepath.Join(tmp, "cache", "testplugin", "1.0.0"))
	req := buildRequest(t, tmp, p)

	if _, err := fork.Execute(req); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	ff, err := forks.Load(req.ForksPath)
	if err != nil {
		t.Fatalf("forks.Load: %v", err)
	}
	if !ff.PluginForked("testplugin@testmarket") {
		t.Error("forks.json not updated for testplugin@testmarket")
	}
	f := ff.Forks["testplugin@testmarket"]
	if f.ForkFullName != "testplugin@agentcfg-forks" {
		t.Errorf("ForkFullName: got %q", f.ForkFullName)
	}
	if f.SourceVersion != "deadbeef1234" {
		t.Errorf("SourceVersion: got %q", f.SourceVersion)
	}
}

func TestExecute_AlreadyForked_Errors(t *testing.T) {
	tmp := t.TempDir()
	p := makePlugin(t, filepath.Join(tmp, "cache", "testplugin", "1.0.0"))
	req := buildRequest(t, tmp, p)

	// First fork succeeds.
	if _, err := fork.Execute(req); err != nil {
		t.Fatalf("first Execute: %v", err)
	}

	// Re-seed settings.json so SetPluginEnabled doesn't fail.
	mkfile(t, req.SettingsPath, `{"enabledPlugins":{"testplugin@testmarket":true}}`)

	// Second fork should fail because the bundle dir already exists.
	if _, err := fork.Execute(req); err == nil {
		t.Error("expected error when forking an already-forked plugin")
	}
}
