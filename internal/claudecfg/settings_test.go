package claudecfg_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jorgenosberg/agentcfg/internal/claudecfg"
)

func TestSetPluginEnabled_DisablesPlugin(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "settings.json")

	initial := map[string]any{
		"model": "opus",
		"enabledPlugins": map[string]bool{
			"alpha@official": true,
			"beta@official":  true,
		},
		"hooks": map[string]any{"PreToolUse": []any{}},
	}
	raw, _ := json.MarshalIndent(initial, "", "  ")
	os.WriteFile(path, raw, 0o644)

	if err := claudecfg.SetPluginEnabled(path, "alpha@official", false); err != nil {
		t.Fatalf("SetPluginEnabled: %v", err)
	}

	data, _ := os.ReadFile(path)
	var result map[string]json.RawMessage
	json.Unmarshal(data, &result)

	var enabled map[string]bool
	json.Unmarshal(result["enabledPlugins"], &enabled)

	if enabled["alpha@official"] {
		t.Error("alpha should be disabled")
	}
	if !enabled["beta@official"] {
		t.Error("beta should remain enabled")
	}

	// Ensure other fields are preserved.
	var model string
	json.Unmarshal(result["model"], &model)
	if model != "opus" {
		t.Errorf("model field was lost: got %q", model)
	}
	if _, ok := result["hooks"]; !ok {
		t.Error("hooks field was lost")
	}
}

func TestSetPluginEnabled_CreatesFileIfMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := claudecfg.SetPluginEnabled(path, "new@market", false); err != nil {
		t.Fatalf("SetPluginEnabled on missing file: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

func TestSetPluginEnabled_AddsNewPlugin(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "settings.json")
	os.WriteFile(path, []byte(`{"enabledPlugins":{"alpha@official":true}}`), 0o644)

	if err := claudecfg.SetPluginEnabled(path, "beta@official", false); err != nil {
		t.Fatalf("SetPluginEnabled: %v", err)
	}

	data, _ := os.ReadFile(path)
	var result map[string]json.RawMessage
	json.Unmarshal(data, &result)
	var enabled map[string]bool
	json.Unmarshal(result["enabledPlugins"], &enabled)

	if !enabled["alpha@official"] {
		t.Error("alpha should still be enabled")
	}
	if enabled["beta@official"] {
		t.Error("beta should be disabled")
	}
}
