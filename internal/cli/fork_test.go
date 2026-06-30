package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildForkFixture creates a sandboxed claude plugin environment under home
// with one installed plugin ("myplugin@market") containing one skill and one
// hook. Returns the plugin's install directory.
//
// Directory layout created:
//
//	<home>/.claude/settings.json           — enabledPlugins: {myplugin@market: true}
//	<home>/.claude/plugins/installed_plugins.json
//	<home>/.claude/plugins/plugin-catalog-cache.json
//	<home>/.claude/plugins/cache/market/myplugin/1.0.0/skills/my-skill/SKILL.md
//	<home>/.claude/plugins/cache/market/myplugin/1.0.0/hooks/my-hook.sh
func buildForkFixture(t *testing.T, home string) string {
	t.Helper()

	claudeDir := filepath.Join(home, ".claude")
	pluginsDir := filepath.Join(claudeDir, "plugins")
	installDir := filepath.Join(pluginsDir, "cache", "market", "myplugin", "1.0.0")

	mkfile(t, filepath.Join(claudeDir, "settings.json"),
		`{"enabledPlugins":{"myplugin@market":true}}`)

	// skill directory
	mkfile(t, filepath.Join(installDir, "skills", "my-skill", "SKILL.md"), "# my skill")
	// hook file
	mkfile(t, filepath.Join(installDir, "hooks", "my-hook.sh"), "#!/bin/sh\necho hook")

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

	out, err := runCLI(t, "fork", "myplugin@market", "--full")
	if err != nil {
		t.Fatalf("fork: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "forked") {
		t.Errorf("expected 'forked' in output: %s", out)
	}
	if !strings.Contains(out, "my-skill") {
		t.Errorf("expected skill name in output: %s", out)
	}

	// Skill should be copied into source tree.
	dest := filepath.Join(cfg.Source, "skills", "my-skill")
	if fi, err := os.Stat(dest); err != nil || !fi.IsDir() {
		t.Errorf("forked skill not found at %s: %v", dest, err)
	}

	// Hook should also be copied (--full).
	hookDest := filepath.Join(cfg.Source, "hooks", "my-hook.sh")
	if _, err := os.Stat(hookDest); err != nil {
		t.Errorf("forked hook not found at %s: %v", hookDest, err)
	}

	// forks.json should record the fork.
	forksPath := filepath.Join(home, ".agentcfg", "forks.json")
	if _, err := os.Stat(forksPath); err != nil {
		t.Errorf("forks.json not created: %v", err)
	}

	// Plugin should be disabled in settings.
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	raw, _ := os.ReadFile(settingsPath)
	if strings.Contains(string(raw), `"myplugin@market":true`) {
		t.Error("plugin should be disabled in settings after fork")
	}
}

func TestForkCmd_SkillOnly(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)
	buildForkFixture(t, home)

	out, err := runCLI(t, "fork", "myplugin@market", "--skill", "my-skill")
	if err != nil {
		t.Fatalf("fork --skill: %v\noutput: %s", err, out)
	}

	// Skill copied.
	dest := filepath.Join(cfg.Source, "skills", "my-skill")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("forked skill not found at %s: %v", dest, err)
	}
	// Hook NOT copied (scope is skill-only).
	hookDest := filepath.Join(cfg.Source, "hooks", "my-hook.sh")
	if _, err := os.Stat(hookDest); err == nil {
		t.Error("hook should not be copied in skill-only fork")
	}
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
	dest := filepath.Join(cfg.Source, "skills", "my-skill")
	if _, err := os.Stat(dest); err == nil {
		t.Error("dry-run must not create files")
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

func TestForkListCmd_ShowsForkedPlugin(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)
	buildForkFixture(t, home)

	// Fork first.
	if _, err := runCLI(t, "fork", "myplugin@market", "--full"); err != nil {
		t.Fatalf("fork: %v", err)
	}

	out, err := runCLI(t, "fork", "list")
	if err != nil {
		t.Fatalf("fork list: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "myplugin@market") {
		t.Errorf("expected plugin name in fork list: %s", out)
	}
	if !strings.Contains(out, "my-skill") {
		t.Errorf("expected skill name in fork list: %s", out)
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
	if _, err := runCLI(t, "fork", "myplugin@market", "--full"); err != nil {
		t.Fatalf("fork: %v", err)
	}

	// The installed plugin still has the same SHA, so status = up-to-date.
	out, err := runCLI(t, "fork", "status")
	if err != nil {
		t.Fatalf("fork status: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "up-to-date") {
		t.Errorf("expected up-to-date status: %s", out)
	}
}
