package plugins_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jorgenosberg/agentcfg/internal/plugins"
)

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestLoad_EmptyWhenMissing(t *testing.T) {
	// Point DefaultDir at a nonexistent path by setting HOME to an empty dir.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	reg, err := plugins.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reg.Plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(reg.Plugins))
	}
}

func TestLoad_MergesThreeFiles(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	claudeDir := filepath.Join(tmp, ".claude")
	pluginsDir := filepath.Join(claudeDir, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// settings.json — one enabled, one disabled
	writeJSON(t, filepath.Join(claudeDir, "settings.json"), map[string]any{
		"enabledPlugins": map[string]bool{
			"alpha@official": true,
			"beta@official":  false,
		},
		"model": "opus",
	})

	// installed_plugins.json
	writeJSON(t, filepath.Join(pluginsDir, "installed_plugins.json"), map[string]any{
		"version": 2,
		"plugins": map[string]any{
			"alpha@official": []map[string]any{{
				"scope":       "user",
				"installPath": "/fake/cache/alpha/1.0.0",
				"version":     "1.0.0",
			}},
			"beta@official": []map[string]any{{
				"scope":       "project",
				"projectPath": tmp,
				"installPath": "/fake/cache/beta/unknown",
				"version":     "unknown",
			}},
		},
	})

	// plugin-catalog-cache.json
	writeJSON(t, filepath.Join(pluginsDir, "plugin-catalog-cache.json"), map[string]any{
		"version": 1,
		"catalog": map[string]any{
			"plugins": map[string]any{
				"alpha@official": map[string]any{
					"components": map[string]any{
						"skills": []map[string]any{{"name": "skill-a"}, {"name": "skill-b"}},
						"hooks":  []any{},
						"mcpServers": []map[string]any{{"name": "mcp-x"}},
						"lspServers": []any{},
					},
				},
				"beta@official": map[string]any{
					"components": map[string]any{
						"skills":     []any{},
						"hooks":      []map[string]any{{"name": "hook-b"}},
						"mcpServers": []any{},
						"lspServers": []any{},
					},
				},
			},
		},
	})

	reg, err := plugins.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(reg.Plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(reg.Plugins))
	}

	alpha, ok := reg.Get("alpha@official")
	if !ok {
		t.Fatal("alpha@official not found")
	}
	if !alpha.Enabled {
		t.Error("alpha should be enabled")
	}
	if alpha.InstallPath != "/fake/cache/alpha/1.0.0" {
		t.Errorf("unexpected installPath: %s", alpha.InstallPath)
	}
	if len(alpha.Skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(alpha.Skills))
	}
	if len(alpha.MCPServers) != 1 || alpha.MCPServers[0] != "mcp-x" {
		t.Errorf("unexpected MCPServers: %v", alpha.MCPServers)
	}

	beta, ok := reg.Get("beta@official")
	if !ok {
		t.Fatal("beta@official not found")
	}
	if beta.Enabled {
		t.Error("beta should be disabled")
	}
	if len(beta.Hooks) != 1 || beta.Hooks[0] != "hook-b" {
		t.Errorf("unexpected hooks: %v", beta.Hooks)
	}
}

func TestLoad_DefaultEnabledWhenAbsentFromSettings(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	claudeDir := filepath.Join(tmp, ".claude")
	pluginsDir := filepath.Join(claudeDir, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeJSON(t, filepath.Join(claudeDir, "settings.json"), map[string]any{
		"enabledPlugins": map[string]bool{},
	})
	writeJSON(t, filepath.Join(pluginsDir, "installed_plugins.json"), map[string]any{
		"version": 2,
		"plugins": map[string]any{
			"gamma@market": []map[string]any{{
				"scope":       "user",
				"installPath": "/fake/gamma",
				"version":     "1.0",
			}},
		},
	})
	writeJSON(t, filepath.Join(pluginsDir, "plugin-catalog-cache.json"), map[string]any{
		"version": 1,
		"catalog": map[string]any{
			"plugins": map[string]any{
				"gamma@market": map[string]any{
					"components": map[string]any{
						"skills": []any{}, "hooks": []any{},
						"mcpServers": []any{}, "lspServers": []any{},
					},
				},
			},
		},
	})

	reg, err := plugins.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	gamma, ok := reg.Get("gamma@market")
	if !ok {
		t.Fatal("gamma@market not found")
	}
	if !gamma.Enabled {
		t.Error("plugin absent from enabledPlugins should default to enabled")
	}
}
