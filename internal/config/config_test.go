package config_test

import (
	"strings"
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

func TestSupportsKindCommand(t *testing.T) {
	claude := config.Target{Name: "claude", Path: "/tmp/claude", Agent: "claude"}
	if !claude.SupportsKind(source.KindCommand) {
		t.Error("claude profile should support command")
	}
	codex := config.Target{Name: "codex", Path: "/tmp/codex", Agent: "codex"}
	if codex.SupportsKind(source.KindCommand) {
		t.Error("codex profile should not support command")
	}
}

func TestSupportsKindRule(t *testing.T) {
	cursor := config.Target{Name: "cursor", Path: "/tmp/cursor", Agent: "cursor"}
	if !cursor.SupportsKind(source.KindRule) {
		t.Error("cursor profile should support rule")
	}
	claude := config.Target{Name: "claude", Path: "/tmp/claude", Agent: "claude"}
	if claude.SupportsKind(source.KindRule) {
		t.Error("claude profile should not support rule")
	}
}

func TestSubdirForCommand(t *testing.T) {
	claude := config.Target{Name: "claude", Path: "/tmp/claude", Agent: "claude"}
	if got := claude.SubdirFor(source.KindCommand); got != "commands" {
		t.Errorf("SubdirFor command on claude: want %q got %q", "commands", got)
	}
}

func TestSubdirForRuleCursor(t *testing.T) {
	// cursor uses the multi-file directory form
	cursor := config.Target{Name: "cursor", Path: "/tmp/cursor", Agent: "cursor"}
	if got := cursor.SubdirFor(source.KindRule); got != ".cursor/rules" {
		t.Errorf("SubdirFor rule on cursor: want %q got %q", ".cursor/rules", got)
	}
}

func TestDestNameFor_NoOverride(t *testing.T) {
	// claude has no DestName override: source name passes through
	claude := config.Target{Name: "claude", Path: "/tmp/claude", Agent: "claude"}
	if got := claude.DestNameFor(source.KindCommand, "review.md"); got != "review.md" {
		t.Errorf("DestNameFor with no override: want %q got %q", "review.md", got)
	}
}

func TestDestNameFor_ClineOverride(t *testing.T) {
	cline := config.Target{Name: "cline", Path: "/tmp/proj", Agent: "cline"}
	if got := cline.DestNameFor(source.KindRule, "my-rules.md"); got != ".clinerules" {
		t.Errorf("DestNameFor cline rule: want %q got %q", ".clinerules", got)
	}
}

func TestDestNameFor_WindsurfOverride(t *testing.T) {
	ws := config.Target{Name: "windsurf", Path: "/tmp/proj", Agent: "windsurf"}
	if got := ws.DestNameFor(source.KindRule, "my-rules.md"); got != ".windsurfrules" {
		t.Errorf("DestNameFor windsurf rule: want %q got %q", ".windsurfrules", got)
	}
}

func TestSupportedSubdirs_FromProfile(t *testing.T) {
	claude := config.Target{Name: "claude", Path: "/tmp/claude", Agent: "claude"}
	sd := claude.SupportedSubdirs()
	if _, ok := sd[source.KindSkill]; !ok {
		t.Error("claude SupportedSubdirs should include skill")
	}
	if _, ok := sd[source.KindCommand]; !ok {
		t.Error("claude SupportedSubdirs should include command")
	}
	if _, ok := sd[source.KindRule]; ok {
		t.Error("claude SupportedSubdirs should not include rule")
	}
}

func TestSupportedSubdirs_ExplicitOverride(t *testing.T) {
	tgt := config.Target{
		Name:    "custom",
		Path:    "/tmp/custom",
		Agent:   "claude",
		Subdirs: map[string]string{source.KindSkill: "skills"},
	}
	sd := tgt.SupportedSubdirs()
	if len(sd) != 1 {
		t.Errorf("explicit Subdirs: expected 1 entry, got %d", len(sd))
	}
	if _, ok := sd[source.KindSkill]; !ok {
		t.Error("explicit Subdirs: skill should be included")
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

// ---------------------------------------------------------------------------
// ResolveStrategy
// ---------------------------------------------------------------------------

func TestResolveStrategy_Explicit(t *testing.T) {
	tgt := config.Target{Strategy: config.StrategyCopy}
	if got := tgt.ResolveStrategy(config.StrategyLink); got != config.StrategyCopy {
		t.Errorf("explicit strategy: want %q, got %q", config.StrategyCopy, got)
	}
}

func TestResolveStrategy_Fallback(t *testing.T) {
	tgt := config.Target{} // no explicit strategy
	if got := tgt.ResolveStrategy(config.StrategyCopy); got != config.StrategyCopy {
		t.Errorf("fallback strategy: want %q, got %q", config.StrategyCopy, got)
	}
}

func TestResolveStrategy_BothEmpty(t *testing.T) {
	tgt := config.Target{}
	if got := tgt.ResolveStrategy(""); got != config.StrategyLink {
		t.Errorf("both empty: want %q (default), got %q", config.StrategyLink, got)
	}
}

// ---------------------------------------------------------------------------
// Excludes
// ---------------------------------------------------------------------------

func TestExcludes_MatchByName(t *testing.T) {
	tgt := config.Target{Exclude: []string{"GEMINI.md"}}
	item := source.Item{Kind: source.KindContext, Name: "GEMINI.md"}
	if !tgt.Excludes(item) {
		t.Error("expected Excludes to match by plain name")
	}
}

func TestExcludes_MatchByKindQualified(t *testing.T) {
	tgt := config.Target{Exclude: []string{"context/GEMINI.md"}}
	item := source.Item{Kind: source.KindContext, Name: "GEMINI.md"}
	if !tgt.Excludes(item) {
		t.Error("expected Excludes to match by kind/name")
	}
}

func TestExcludes_NoMatch(t *testing.T) {
	tgt := config.Target{Exclude: []string{"context/GEMINI.md"}}
	other := source.Item{Kind: source.KindContext, Name: "CLAUDE.md"}
	if tgt.Excludes(other) {
		t.Error("Excludes should not match a different item")
	}
}

func TestExcludes_KindMismatch(t *testing.T) {
	// "skill/GEMINI.md" should not match a context item of same name.
	tgt := config.Target{Exclude: []string{"skill/GEMINI.md"}}
	item := source.Item{Kind: source.KindContext, Name: "GEMINI.md"}
	if tgt.Excludes(item) {
		t.Error("Excludes with kind prefix should not match a different kind")
	}
}

// ---------------------------------------------------------------------------
// Default / DefaultSource / DefaultPath
// ---------------------------------------------------------------------------

func TestDefault_Shape(t *testing.T) {
	cfg := config.Default("")
	if cfg.DefaultStrategy != config.StrategyLink {
		t.Errorf("Default strategy: want %q, got %q", config.StrategyLink, cfg.DefaultStrategy)
	}
	if cfg.Projects == nil {
		t.Error("Default Projects should be non-nil slice")
	}
	if cfg.Targets == nil {
		t.Error("Default Targets should be non-nil slice")
	}
}

func TestDefault_CustomSource(t *testing.T) {
	cfg := config.Default("/custom/source")
	if cfg.Source != "/custom/source" {
		t.Errorf("Default with custom source: want %q, got %q", "/custom/source", cfg.Source)
	}
}

func TestDefaultSource_Sandboxed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGENTCFG_HOME", home)

	src, err := config.DefaultSource()
	if err != nil {
		t.Fatalf("DefaultSource: %v", err)
	}
	if !strings.HasPrefix(src, home) {
		t.Errorf("DefaultSource should be under sandbox %q, got %q", home, src)
	}
}

func TestDefaultPath_Sandboxed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGENTCFG_HOME", home)

	p, err := config.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if !strings.HasPrefix(p, home) {
		t.Errorf("DefaultPath should be under sandbox %q, got %q", home, p)
	}
}
