package config_test

import (
	"testing"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/source"
)

func TestIsDisabled(t *testing.T) {
	tgt := config.Target{
		Name:     "claude",
		Path:     "/tmp/claude",
		Disabled: []string{"my-skill"},
	}
	skill := source.Item{Kind: source.KindSkill, Name: "my-skill"}
	other := source.Item{Kind: source.KindSkill, Name: "other-skill"}
	if !tgt.IsDisabled(skill) {
		t.Error("expected my-skill to be disabled")
	}
	if tgt.IsDisabled(other) {
		t.Error("expected other-skill not to be disabled")
	}
}

func TestIsDisabledKindQualified(t *testing.T) {
	tgt := config.Target{
		Name:     "claude",
		Path:     "/tmp/claude",
		Disabled: []string{"skill/my-skill"},
	}
	skill := source.Item{Kind: source.KindSkill, Name: "my-skill"}
	hook := source.Item{Kind: source.KindHook, Name: "my-skill"}
	if !tgt.IsDisabled(skill) {
		t.Error("expected skill/my-skill to match")
	}
	if tgt.IsDisabled(hook) {
		t.Error("expected hook/my-skill not to match skill/my-skill qualifier")
	}
}

func TestSupportsKindWithAgentProfile(t *testing.T) {
	tgt := config.Target{Name: "claude-work", Path: "/tmp/cw", Agent: "claude"}
	if !tgt.SupportsKind("skill") {
		t.Error("claude profile should support skill")
	}
	if !tgt.SupportsKind("hook") {
		t.Error("claude profile should support hook")
	}
	if tgt.SupportsKind("rule") {
		t.Error("claude profile should not support rule")
	}
}

func TestSupportsKindExplicitSubdirsOverrideProfile(t *testing.T) {
	tgt := config.Target{
		Name:    "custom",
		Path:    "/tmp/custom",
		Agent:   "claude",
		Subdirs: map[string]string{"skill": "skills"},
	}
	if !tgt.SupportsKind("skill") {
		t.Error("explicit subdirs: skill should be supported")
	}
	if tgt.SupportsKind("hook") {
		t.Error("explicit subdirs: hook should not be supported (not in Subdirs)")
	}
}

func TestSubdirForWithAgentProfile(t *testing.T) {
	tgt := config.Target{Name: "claude-work", Path: "/tmp/cw", Agent: "claude"}
	if got := tgt.SubdirFor("skill"); got != "skills" {
		t.Errorf("SubdirFor skill: want %q got %q", "skills", got)
	}
	if got := tgt.SubdirFor("hook"); got != "hooks" {
		t.Errorf("SubdirFor hook: want %q got %q", "hooks", got)
	}
	if got := tgt.SubdirFor("context"); got != "" {
		t.Errorf("SubdirFor context: want %q got %q", "", got)
	}
}

func TestAliasRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/config.json"

	cfg := config.Config{
		Source:          dir,
		DefaultStrategy: config.StrategyLink,
		Targets: []config.Target{
			{Name: "claude-personal", Path: "/tmp/cp", Agent: "claude", Alias: "claude"},
			{Name: "claude-work", Path: "/tmp/cw", Agent: "claude", Alias: "claude"},
			{Name: "codex", Path: "/tmp/codex", Agent: "codex"}, // no alias
		},
	}
	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Targets) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(loaded.Targets))
	}
	if loaded.Targets[0].Alias != "claude" {
		t.Errorf("targets[0].Alias: want %q got %q", "claude", loaded.Targets[0].Alias)
	}
	if loaded.Targets[1].Alias != "claude" {
		t.Errorf("targets[1].Alias: want %q got %q", "claude", loaded.Targets[1].Alias)
	}
	if loaded.Targets[2].Alias != "" {
		t.Errorf("targets[2].Alias: want empty, got %q", loaded.Targets[2].Alias)
	}
}

func TestSubdirForExplicitSubdirsNoFallthrough(t *testing.T) {
	// When Subdirs is explicitly set, SubdirFor must not fall through to the
	// agent profile for missing keys. SupportsKind and SubdirFor must agree.
	tgt := config.Target{
		Name:    "custom",
		Path:    "/tmp/custom",
		Agent:   "claude",
		Subdirs: map[string]string{"skill": "skills"},
	}
	// hook is not in Subdirs; SupportsKind returns false for it
	if tgt.SupportsKind("hook") {
		t.Error("hook should not be supported with explicit Subdirs that omits it")
	}
	// SubdirFor should also return "" (not the claude profile's "hooks")
	if got := tgt.SubdirFor("hook"); got != "" {
		t.Errorf("SubdirFor hook with explicit Subdirs: want %q got %q", "", got)
	}
}
