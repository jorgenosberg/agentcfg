package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/source"
)

func moveFixture(t *testing.T) (config.Config, string, config.Project) {
	t.Helper()
	root := t.TempDir()
	src := filepath.Join(root, "source")
	skill := filepath.Join(src, "skills", "demo")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("demo"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyLink,
		Targets: []config.Target{
			{Name: "claude", Agent: "claude", Path: filepath.Join(root, "home", ".claude")},
			{Name: "agents", Agent: "agents", Path: filepath.Join(root, "home", ".agents")},
		},
	}
	proj := config.Project{Name: "repo", Path: filepath.Join(root, "repo")}
	if err := os.MkdirAll(proj.Path, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg.Projects = []config.Project{proj}
	cfgPath := filepath.Join(root, "config.json")
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	return cfg, cfgPath, proj
}

func TestMoveToProjectAndBack(t *testing.T) {
	cfg, cfgPath, proj := moveFixture(t)
	it := source.Item{Kind: source.KindSkill, Name: "demo", Path: filepath.Join(cfg.Source, "skills", "demo")}
	for _, tg := range cfg.Targets {
		if _, err := Install(tg, config.StrategyLink, it); err != nil {
			t.Fatal(err)
		}
	}

	plan, err := PlanToProject(cfg, it, proj, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(cfgPath, false); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".claude/skills/demo/SKILL.md", ".agents/skills/demo/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(proj.Path, rel)); err != nil {
			t.Errorf("missing project install %s: %v", rel, err)
		}
	}
	for _, tg := range cfg.Targets {
		if _, err := os.Lstat(destPath(tg, it)); !os.IsNotExist(err) {
			t.Errorf("global install on %s should be gone", tg.Name)
		}
	}
	reloaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, tg := range reloaded.Targets {
		if !tg.IsDisabled(it) {
			t.Errorf("%s should have demo disabled", tg.Name)
		}
	}
	if _, err := os.Stat(filepath.Join(it.Path, "SKILL.md")); err != nil {
		t.Errorf("source item must survive: %v", err)
	}

	// Back to global: edit the project copy first, then promote it.
	edited := filepath.Join(proj.Path, ".claude/skills/demo/SKILL.md")
	if err := os.WriteFile(edited, []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj.Path, ".agents/skills/demo/SKILL.md"), []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	scanned, _ := source.ScanProject(proj.Path, proj.Name)
	var copies []source.ProjectItem
	for _, pi := range scanned {
		if pi.Name == "demo" {
			copies = append(copies, pi)
		}
	}
	gp, err := PlanToGlobal(reloaded, proj, copies, reloaded.Targets)
	if err != nil {
		t.Fatal(err)
	}
	if err := gp.Apply(cfgPath, reloaded, false, false); err == nil {
		t.Fatal("expected refusal: source has different content")
	}
	if err := gp.Apply(cfgPath, reloaded, true, false); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(it.Path, "SKILL.md"))
	if string(got) != "edited" {
		t.Errorf("source content = %q, want edited", got)
	}
	for _, tg := range reloaded.Targets {
		if st := statusOf(destPath(tg, it), it.Path, config.StrategyLink); st != StatusLinked {
			t.Errorf("%s status = %s, want linked", tg.Name, st)
		}
	}
	if _, err := os.Stat(filepath.Join(proj.Path, ".claude/skills/demo")); !os.IsNotExist(err) {
		t.Errorf("project copy should be removed")
	}
}

func TestMoveToProjectKeepsDifferingFile(t *testing.T) {
	cfg, cfgPath, proj := moveFixture(t)
	it := source.Item{Kind: source.KindSkill, Name: "demo", Path: filepath.Join(cfg.Source, "skills", "demo")}
	if _, err := Install(cfg.Targets[0], config.StrategyLink, it); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(proj.Path, ".claude/skills/demo")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "SKILL.md"), []byte("project-own"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanToProject(cfg, it, proj, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(cfgPath, false); err == nil {
		t.Fatal("expected conflict error")
	}
	if _, err := os.Lstat(destPath(cfg.Targets[0], it)); err != nil {
		t.Errorf("global install must stay when the move fails: %v", err)
	}
}

func TestMoveBackFromLinkedProjectKeepsSource(t *testing.T) {
	cfg, cfgPath, proj := moveFixture(t)
	it := source.Item{Kind: source.KindSkill, Name: "demo", Path: filepath.Join(cfg.Source, "skills", "demo")}
	if _, err := Install(cfg.Targets[0], config.StrategyLink, it); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanToProject(cfg, it, proj, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(cfgPath, false); err != nil {
		t.Fatal(err)
	}
	reloaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	scanned, err := source.ScanProject(proj.Path, proj.Name)
	if err != nil {
		t.Fatal(err)
	}
	gp, err := PlanToGlobal(reloaded, proj, scanned, reloaded.Targets)
	if err != nil {
		t.Fatal(err)
	}
	if err := gp.Apply(cfgPath, reloaded, true, false); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(it.Path, "SKILL.md"))
	if err != nil || string(got) != "demo" {
		t.Fatalf("source item lost: %q, %v", got, err)
	}
}

func TestMoveRejectsContext(t *testing.T) {
	cfg, _, proj := moveFixture(t)
	it := source.Item{Kind: source.KindContext, Name: "AGENTS.md", Path: filepath.Join(cfg.Source, "AGENTS.md")}
	if _, err := PlanToProject(cfg, it, proj, false); err == nil {
		t.Error("expected PlanToProject to reject context")
	}
	copies := []source.ProjectItem{{Kind: source.KindContext, Name: "AGENTS.md", Path: filepath.Join(proj.Path, "AGENTS.md")}}
	if _, err := PlanToGlobal(cfg, proj, copies, cfg.Targets); err == nil {
		t.Error("expected PlanToGlobal to reject context")
	}
}

func TestMoveToProjectKeepsTargetsWithoutProjectLocation(t *testing.T) {
	cfg, cfgPath, proj := moveFixture(t)
	cmdDir := filepath.Join(cfg.Source, "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "mycmd.md"), []byte("cmd"), 0o644); err != nil {
		t.Fatal(err)
	}
	it := source.Item{Kind: source.KindCommand, Name: "mycmd.md", Path: filepath.Join(cmdDir, "mycmd.md")}
	// Codex has no project-level commands; a custom Subdirs gives it global ones.
	cfg.Targets[1] = config.Target{Name: "codex", Agent: "codex", Path: filepath.Join(filepath.Dir(cfgPath), "home", ".codex"), Subdirs: map[string]string{source.KindCommand: "prompts"}}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	for _, tg := range cfg.Targets {
		if _, err := Install(tg, config.StrategyLink, it); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := PlanToProject(cfg, it, proj, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Kept) != 1 || plan.Kept[0].Target.Name != "codex" {
		t.Fatalf("Kept = %+v, want codex", plan.Kept)
	}
	if err := plan.Apply(cfgPath, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(destPath(cfg.Targets[1], it)); err != nil {
		t.Errorf("codex global install should stay: %v", err)
	}
	reloaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Targets[1].IsDisabled(it) {
		t.Error("codex should not be disabled")
	}
	if !reloaded.Targets[0].IsDisabled(it) {
		t.Error("claude should be disabled")
	}
}
