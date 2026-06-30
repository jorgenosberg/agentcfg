package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// assertPluginEnabled reads settingsPath and asserts the named plugin's enabled
// state matches want, parsing the JSON rather than doing string matching.
func assertPluginEnabled(t *testing.T, settingsPath, pluginName string, want bool) {
	t.Helper()
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	var doc struct {
		EnabledPlugins map[string]bool `json:"enabledPlugins"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse settings.json: %v", err)
	}
	if got := doc.EnabledPlugins[pluginName]; got != want {
		t.Errorf("enabledPlugins[%q]: got %v want %v (settings: %s)", pluginName, got, want, raw)
	}
}

// buildForkFixture creates a sandboxed Claude plugin environment with one
// installed plugin ("myplugin@market") containing a full bundle.
//
// Directory layout created:
//
//	<home>/.claude/settings.json                     — enabledPlugins: {myplugin@market: true}
//	<home>/.claude/plugins/installed_plugins.json
//	<home>/.claude/plugins/plugin-catalog-cache.json
//	<home>/.claude/plugins/known_marketplaces.json   — empty {}
//	<home>/.claude/plugins/cache/market/myplugin/1.0.0/skills/my-skill/SKILL.md
//	<home>/.claude/plugins/cache/market/myplugin/1.0.0/commands/review.md
//	<home>/.claude/plugins/cache/market/myplugin/1.0.0/agents/helper.md
//	<home>/.claude/plugins/cache/market/myplugin/1.0.0/hooks/my-hook.sh
//	<home>/.claude/plugins/cache/market/myplugin/1.0.0/bin/tool
func buildForkFixture(t *testing.T, home string) string {
	t.Helper()

	claudeDir := filepath.Join(home, ".claude")
	pluginsDir := filepath.Join(claudeDir, "plugins")
	installDir := filepath.Join(pluginsDir, "cache", "market", "myplugin", "1.0.0")

	mkfile(t, filepath.Join(claudeDir, "settings.json"),
		`{"enabledPlugins":{"myplugin@market":true}}`)

	// Full bundle: skills, commands, agents, hooks, bin
	mkfile(t, filepath.Join(installDir, "skills", "my-skill", "SKILL.md"), "# my skill")
	mkfile(t, filepath.Join(installDir, "commands", "review.md"), "# review command")
	mkfile(t, filepath.Join(installDir, "agents", "helper.md"), "# agent")
	mkfile(t, filepath.Join(installDir, "hooks", "my-hook.sh"), "#!/bin/sh\necho hook")
	mkfile(t, filepath.Join(installDir, "bin", "tool"), "#!/bin/sh\necho tool")
	mkfile(t, filepath.Join(installDir, ".claude-plugin", "plugin.json"),
		`{"name":"myplugin","version":"1.0.0"}`)

	installed := map[string]any{
		"version": 2,
		"plugins": map[string]any{
			"myplugin@market": []map[string]any{{
				"scope":        "user",
				"installPath":  installDir,
				"version":      "1.0.0",
				"gitCommitSha": "deadbeef1234",
			}},
		},
	}
	writeJSONFile(t, filepath.Join(pluginsDir, "installed_plugins.json"), installed)

	catalog := map[string]any{
		"version": 1,
		"catalog": map[string]any{
			"plugins": map[string]any{
				"myplugin@market": map[string]any{
					"components": map[string]any{
						"skills":     []map[string]any{{"name": "my-skill"}},
						"hooks":      []map[string]any{{"name": "my-hook.sh"}},
						"mcpServers": []any{},
						"lspServers": []any{},
					},
				},
			},
		},
	}
	writeJSONFile(t, filepath.Join(pluginsDir, "plugin-catalog-cache.json"), catalog)

	// Empty known_marketplaces.json so RegisterMarketplace has a file to update.
	writeJSONFile(t, filepath.Join(pluginsDir, "known_marketplaces.json"), map[string]any{})

	return installDir
}

// writeJSONFile serialises v as indented JSON to path (creating parent dirs).
func writeJSONFile(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	mkfile(t, path, string(raw))
}

func TestForkCmd_FullFork(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)
	buildForkFixture(t, home)

	out, err := runCLI(t, "fork", "myplugin@market")
	if err != nil {
		t.Fatalf("fork: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "forked") {
		t.Errorf("expected 'forked' in output: %s", out)
	}

	// Bundle should be in ~/.agentcfg/forks/plugins/myplugin
	bundleDest := filepath.Join(home, ".agentcfg", "forks", "plugins", "myplugin")
	if fi, err := os.Stat(bundleDest); err != nil || !fi.IsDir() {
		t.Errorf("bundle dir not found at %s: %v", bundleDest, err)
	}

	// commands/ and agents/ must be copied too (not just skills/hooks).
	for _, rel := range []string{
		"skills/my-skill/SKILL.md",
		"commands/review.md",
		"agents/helper.md",
		"hooks/my-hook.sh",
		"bin/tool",
	} {
		if _, err := os.Stat(filepath.Join(bundleDest, rel)); err != nil {
			t.Errorf("expected bundle file %s not found: %v", rel, err)
		}
	}

	// forks.json should record the fork.
	forksPath := filepath.Join(home, ".agentcfg", "forks.json")
	if _, err := os.Stat(forksPath); err != nil {
		t.Errorf("forks.json not created: %v", err)
	}

	// Plugin should be disabled in settings; fork enabled.
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	assertPluginEnabled(t, settingsPath, "myplugin@agentcfg-forks", true)
	assertPluginEnabled(t, settingsPath, "myplugin@market", false)
}

func TestForkCmd_DryRun(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)
	buildForkFixture(t, home)

	out, err := runCLI(t, "fork", "myplugin@market", "--dry-run")
	if err != nil {
		t.Fatalf("fork --dry-run: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "dry-run") {
		t.Errorf("expected 'dry-run' in output: %s", out)
	}

	// Nothing should actually be written.
	bundleDest := filepath.Join(home, ".agentcfg", "forks", "plugins", "myplugin")
	if _, err := os.Stat(bundleDest); err == nil {
		t.Error("dry-run must not create the bundle directory")
	}
}

func TestForkCmd_UnknownPlugin(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)
	buildForkFixture(t, home)

	_, err := runCLI(t, "fork", "nosuchplugin@market")
	if err == nil {
		t.Error("expected error for unknown plugin")
	}
}

func TestForkListCmd_EmptyWhenNoForks(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	out, err := runCLI(t, "fork", "list")
	if err != nil {
		t.Fatalf("fork list: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "no forks") {
		t.Errorf("expected 'no forks' for empty forks.json: %s", out)
	}
}

func TestForkListCmd_ShowsBundlePath(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)
	buildForkFixture(t, home)

	// Fork first.
	if _, err := runCLI(t, "fork", "myplugin@market"); err != nil {
		t.Fatalf("fork: %v", err)
	}

	out, err := runCLI(t, "fork", "list")
	if err != nil {
		t.Fatalf("fork list: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "myplugin@market") {
		t.Errorf("expected plugin name in fork list: %s", out)
	}
	// List now shows BUNDLE column (path), not SKILLS.
	if !strings.Contains(out, "forks") {
		t.Errorf("expected bundle path in fork list: %s", out)
	}
}

func TestForkStatusCmd_EmptyWhenNoForks(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	out, err := runCLI(t, "fork", "status")
	if err != nil {
		t.Fatalf("fork status: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "no forks") {
		t.Errorf("expected 'no forks' when no forks recorded: %s", out)
	}
}

func TestForkStatusCmd_UpToDate(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)
	buildForkFixture(t, home)

	// Fork; forks.json will record the SHA "deadbeef1234".
	if _, err := runCLI(t, "fork", "myplugin@market"); err != nil {
		t.Fatalf("fork: %v", err)
	}

	// The installed upstream still has the same SHA → status = up-to-date.
	out, err := runCLI(t, "fork", "status")
	if err != nil {
		t.Fatalf("fork status: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "up-to-date") {
		t.Errorf("expected up-to-date status: %s", out)
	}
}
