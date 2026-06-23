package fork_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jorgenosberg/agentcfg/internal/fork"
	"github.com/jorgenosberg/agentcfg/internal/forks"
	"github.com/jorgenosberg/agentcfg/internal/plugins"
)

func makePlugin(t *testing.T, installDir string) plugins.Plugin {
	t.Helper()
	// Create plugin cache structure: installDir/skills/my-skill/SKILL.md
	skillDir := filepath.Join(installDir, "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: my-skill\n---\nContent."), 0o644)

	hookDir := filepath.Join(installDir, "hooks")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(hookDir, "my-hook.sh"), []byte("#!/bin/sh\necho hello"), 0o755)

	return plugins.Plugin{
		Name:        "testplugin",
		Marketplace: "testmarket",
		FullName:    "testplugin@testmarket",
		Installed:   true,
		Enabled:     true,
		InstallPath: installDir,
		Version:     "1.0.0",
		Skills:      []string{"my-skill"},
		Hooks:       []string{"my-hook.sh"},
		MCPServers:  []string{"mcp-server"},
	}
}

func writeSettings(t *testing.T, path string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(`{"enabledPlugins":{"testplugin@testmarket":true}}`), 0o644)
}

func TestExecute_FullFork(t *testing.T) {
	tmp := t.TempDir()
	installDir := filepath.Join(tmp, "cache", "testplugin", "1.0.0")
	sourceRoot := filepath.Join(tmp, "source")
	forksPath := filepath.Join(tmp, "forks.json")
	settingsPath := filepath.Join(tmp, "settings.json")

	p := makePlugin(t, installDir)
	writeSettings(t, settingsPath)

	res, err := fork.Execute(fork.Request{
		Plugin:       p,
		Scope:        fork.ScopeFull,
		SourceRoot:   sourceRoot,
		ForksPath:    forksPath,
		SettingsPath: settingsPath,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(res.ForkedSkills) != 1 || res.ForkedSkills[0] != "my-skill" {
		t.Errorf("ForkedSkills: %v", res.ForkedSkills)
	}
	if len(res.ForkedHooks) != 1 || res.ForkedHooks[0] != "my-hook.sh" {
		t.Errorf("ForkedHooks: %v", res.ForkedHooks)
	}
	if len(res.Skipped.MCPServers) != 1 || res.Skipped.MCPServers[0] != "mcp-server" {
		t.Errorf("Skipped.MCPServers: %v", res.Skipped.MCPServers)
	}

	// Skill landed in source.
	if _, err := os.Stat(filepath.Join(sourceRoot, "skills", "my-skill", "SKILL.md")); err != nil {
		t.Error("skill file not copied:", err)
	}

	// Hook landed in source.
	if _, err := os.Stat(filepath.Join(sourceRoot, "hooks", "my-hook.sh")); err != nil {
		t.Error("hook file not copied:", err)
	}

	// forks.json written.
	ff, _ := forks.Load(forksPath)
	if !ff.PluginForked("testplugin@testmarket") {
		t.Error("forks.json not updated")
	}

	// Plugin disabled in settings.json.
	data, _ := os.ReadFile(settingsPath)
	var doc map[string]json.RawMessage
	json.Unmarshal(data, &doc)
	var enabled map[string]bool
	json.Unmarshal(doc["enabledPlugins"], &enabled)
	if enabled["testplugin@testmarket"] {
		t.Error("plugin should be disabled in settings.json")
	}
}

func TestExecute_ScopeSkill(t *testing.T) {
	tmp := t.TempDir()
	installDir := filepath.Join(tmp, "cache")
	sourceRoot := filepath.Join(tmp, "source")

	p := makePlugin(t, installDir)
	writeSettings(t, filepath.Join(tmp, "settings.json"))

	res, err := fork.Execute(fork.Request{
		Plugin:       p,
		Scope:        fork.ScopeSkill,
		Skills:       []string{"my-skill"},
		SourceRoot:   sourceRoot,
		ForksPath:    filepath.Join(tmp, "forks.json"),
		SettingsPath: filepath.Join(tmp, "settings.json"),
	})
	if err != nil {
		t.Fatalf("Execute (ScopeSkill): %v", err)
	}

	if len(res.ForkedSkills) != 1 {
		t.Errorf("expected 1 forked skill, got %d", len(res.ForkedSkills))
	}
	if len(res.ForkedHooks) != 0 {
		t.Errorf("ScopeSkill should not copy hooks, got %v", res.ForkedHooks)
	}
}

func TestExecute_CollisionAborts(t *testing.T) {
	tmp := t.TempDir()
	installDir := filepath.Join(tmp, "cache")
	sourceRoot := filepath.Join(tmp, "source")

	p := makePlugin(t, installDir)
	writeSettings(t, filepath.Join(tmp, "settings.json"))

	// Pre-create the destination to trigger collision.
	os.MkdirAll(filepath.Join(sourceRoot, "skills", "my-skill"), 0o755)

	_, err := fork.Execute(fork.Request{
		Plugin:       p,
		Scope:        fork.ScopeFull,
		SourceRoot:   sourceRoot,
		ForksPath:    filepath.Join(tmp, "forks.json"),
		SettingsPath: filepath.Join(tmp, "settings.json"),
	})
	if err == nil {
		t.Error("expected error for skill collision, got nil")
	}
}
