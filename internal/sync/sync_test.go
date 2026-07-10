// internal/sync/sync_test.go
package sync_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/lock"
	"github.com/jorgenosberg/agentcfg/internal/source"
	"github.com/jorgenosberg/agentcfg/internal/sync"
)

func makeSourceItem(t *testing.T, dir, kind, name, content string) source.Item {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write source item: %v", err)
	}
	return source.Item{Kind: kind, Name: name, Path: p}
}

func TestInspectDisabled(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()

	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# hello")
	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyLink,
		Targets: []config.Target{
			{
				Name:     "claude",
				Path:     tgtDir,
				Disabled: []string{"CLAUDE.md"},
			},
		},
	}

	entries := sync.Inspect(cfg, []source.Item{item})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Status != sync.StatusDisabled {
		t.Errorf("expected StatusDisabled, got %s", entries[0].Status)
	}
}

func TestInspectDisabledNotReinstalledBySync(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()

	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# hello")
	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyLink,
		Targets: []config.Target{
			{
				Name:     "claude",
				Path:     tgtDir,
				Disabled: []string{"CLAUDE.md"},
			},
		},
	}

	results := sync.Sync(cfg, []source.Item{item}, nil, false, false)
	if len(results) != 0 {
		t.Errorf("sync should skip disabled items, got %d results", len(results))
	}

	dest := filepath.Join(tgtDir, "CLAUDE.md")
	if _, err := os.Lstat(dest); !os.IsNotExist(err) {
		t.Error("disabled item should not be installed")
	}
}

func TestToggleDisable(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()
	cfgPath := filepath.Join(t.TempDir(), "config.json")

	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# hello")
	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyLink,
		Targets: []config.Target{
			{Name: "claude", Path: tgtDir},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	// Install first so there is something to uninstall
	if _, err := sync.Install(cfg.Targets[0], config.StrategyLink, item); err != nil {
		t.Fatalf("install: %v", err)
	}
	dest := filepath.Join(tgtDir, "CLAUDE.md")
	if _, err := os.Lstat(dest); err != nil {
		t.Fatalf("expected symlink at dest before toggle: %v", err)
	}

	if err := sync.Toggle(cfgPath, "claude", item, true); err != nil {
		t.Fatalf("Toggle disable: %v", err)
	}

	// Symlink should be gone
	if _, err := os.Lstat(dest); !os.IsNotExist(err) {
		t.Error("expected dest to be removed after disable")
	}

	// Config should have item in Disabled
	updated, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	found := false
	for _, d := range updated.Targets[0].Disabled {
		if d == "CLAUDE.md" {
			found = true
		}
	}
	if !found {
		t.Error("expected CLAUDE.md in Disabled after toggle")
	}
}

func TestToggleEnable(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()
	cfgPath := filepath.Join(t.TempDir(), "config.json")

	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# hello")
	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyLink,
		Targets: []config.Target{
			{Name: "claude", Path: tgtDir, Disabled: []string{"CLAUDE.md"}},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if err := sync.Toggle(cfgPath, "claude", item, false); err != nil {
		t.Fatalf("Toggle enable: %v", err)
	}

	dest := filepath.Join(tgtDir, "CLAUDE.md")
	fi, err := os.Lstat(dest)
	if err != nil {
		t.Fatalf("expected symlink at dest after enable: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink at dest")
	}

	updated, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load updated config: %v", err)
	}
	for _, d := range updated.Targets[0].Disabled {
		if d == "CLAUDE.md" {
			t.Error("CLAUDE.md should not be in Disabled after enable")
		}
	}
}

func TestToggleIdempotent(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()
	cfgPath := filepath.Join(t.TempDir(), "config.json")

	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# hello")
	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyLink,
		Targets: []config.Target{
			{Name: "claude", Path: tgtDir, Disabled: []string{"CLAUDE.md"}},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	// Disable again on already-disabled item should not error or duplicate
	if err := sync.Toggle(cfgPath, "claude", item, true); err != nil {
		t.Fatalf("Toggle disable idempotent: %v", err)
	}
	updated, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load updated config: %v", err)
	}
	count := 0
	for _, d := range updated.Targets[0].Disabled {
		if d == "CLAUDE.md" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 disabled entry, got %d", count)
	}
}

func TestUnmanage(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()

	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "hello world")
	tgt := config.Target{Name: "claude", Path: tgtDir}

	// Install as symlink first
	if _, err := sync.Install(tgt, config.StrategyLink, item); err != nil {
		t.Fatalf("install: %v", err)
	}
	dest := filepath.Join(tgtDir, "CLAUDE.md")
	fi, _ := os.Lstat(dest)
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("expected symlink before Unmanage")
	}

	if err := sync.Unmanage(tgt, config.StrategyLink, item); err != nil {
		t.Fatalf("Unmanage: %v", err)
	}

	fi, err := os.Lstat(dest)
	if err != nil {
		t.Fatalf("dest should exist after Unmanage: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Error("dest should be a real file, not a symlink, after Unmanage")
	}
	data, _ := os.ReadFile(dest)
	if string(data) != "hello world" {
		t.Errorf("content mismatch after Unmanage: %q", data)
	}
}

func TestUnmanageIdempotent(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()

	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "hello")
	tgt := config.Target{Name: "claude", Path: tgtDir}

	// Write a real file (already unmanaged)
	dest := filepath.Join(tgtDir, "CLAUDE.md")
	if err := os.WriteFile(dest, []byte("other content"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Should be a no-op
	if err := sync.Unmanage(tgt, config.StrategyLink, item); err != nil {
		t.Fatalf("Unmanage on already-unmanaged should not error: %v", err)
	}
	data, _ := os.ReadFile(dest)
	if string(data) != "other content" {
		t.Error("existing unmanaged content should be preserved")
	}
}

func TestUnmanageAbsent(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()

	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "hello")
	tgt := config.Target{Name: "claude", Path: tgtDir}

	if err := sync.Unmanage(tgt, config.StrategyLink, item); err != nil {
		t.Fatalf("Unmanage on absent dest should not error: %v", err)
	}
	dest := filepath.Join(tgtDir, "CLAUDE.md")
	data, _ := os.ReadFile(dest)
	if string(data) != "hello" {
		t.Errorf("content mismatch: %q", data)
	}
}

// ---------------------------------------------------------------------------
// destPath (tested via Inspect Entry.Dest)
// ---------------------------------------------------------------------------

func TestDestPath_EmptySubdir(t *testing.T) {
	// No agent, no Subdirs → defaultSubdirs["context"] = "" → dest = tgt/name.
	src := t.TempDir()
	tgtDir := t.TempDir()
	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# hello")
	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyLink,
		Targets:         []config.Target{{Name: "t", Path: tgtDir}},
	}
	entries := sync.Inspect(cfg, []source.Item{item})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	want := filepath.Join(tgtDir, "CLAUDE.md")
	if entries[0].Dest != want {
		t.Errorf("destPath: want %q, got %q", want, entries[0].Dest)
	}
}

func TestDestPath_NonEmptySubdir(t *testing.T) {
	// No agent, no Subdirs → defaultSubdirs["skill"] = "skills" → dest = tgt/skills/name.
	src := t.TempDir()
	tgtDir := t.TempDir()
	// Skill items are directories; create one.
	skillPath := filepath.Join(src, "my-skill")
	if err := os.MkdirAll(skillPath, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	item := source.Item{Kind: source.KindSkill, Name: "my-skill", Path: skillPath}

	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyLink,
		Targets:         []config.Target{{Name: "t", Path: tgtDir}},
	}
	entries := sync.Inspect(cfg, []source.Item{item})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	want := filepath.Join(tgtDir, "skills", "my-skill")
	if entries[0].Dest != want {
		t.Errorf("destPath: want %q, got %q", want, entries[0].Dest)
	}
}

// ---------------------------------------------------------------------------
// statusOf branches (tested via Inspect Entry.Status)
// ---------------------------------------------------------------------------

func TestInspect_Absent(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()
	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# hello")
	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyLink,
		Targets:         []config.Target{{Name: "t", Path: tgtDir}},
	}
	entries := sync.Inspect(cfg, []source.Item{item})
	if entries[0].Status != sync.StatusAbsent {
		t.Errorf("want StatusAbsent, got %q", entries[0].Status)
	}
}

func TestInspect_Linked(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()
	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# hello")
	tgt := config.Target{Name: "t", Path: tgtDir}

	if _, err := sync.Install(tgt, config.StrategyLink, item); err != nil {
		t.Fatalf("install: %v", err)
	}

	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyLink,
		Targets:         []config.Target{tgt},
	}
	entries := sync.Inspect(cfg, []source.Item{item})
	if entries[0].Status != sync.StatusLinked {
		t.Errorf("want StatusLinked, got %q", entries[0].Status)
	}
}

func TestInspect_Unmanaged_PlainFile(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()
	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# hello")

	// Write a plain file at dest that is NOT a symlink to the source.
	dest := filepath.Join(tgtDir, "CLAUDE.md")
	if err := os.WriteFile(dest, []byte("other"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyLink,
		Targets:         []config.Target{{Name: "t", Path: tgtDir}},
	}
	entries := sync.Inspect(cfg, []source.Item{item})
	if entries[0].Status != sync.StatusUnmanaged {
		t.Errorf("plain file → want StatusUnmanaged, got %q", entries[0].Status)
	}
}

func TestInspect_Unmanaged_WrongSymlink(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()
	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# hello")

	// Symlink pointing somewhere other than the source item.
	other := filepath.Join(src, "other.md")
	if err := os.WriteFile(other, []byte("other"), 0o644); err != nil {
		t.Fatalf("write other: %v", err)
	}
	dest := filepath.Join(tgtDir, "CLAUDE.md")
	if err := os.Symlink(other, dest); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyLink,
		Targets:         []config.Target{{Name: "t", Path: tgtDir}},
	}
	entries := sync.Inspect(cfg, []source.Item{item})
	if entries[0].Status != sync.StatusUnmanaged {
		t.Errorf("wrong symlink → want StatusUnmanaged, got %q", entries[0].Status)
	}
}

func TestInspect_Copied(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()
	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# hello")
	tgt := config.Target{Name: "t", Path: tgtDir}

	if _, err := sync.Install(tgt, config.StrategyCopy, item); err != nil {
		t.Fatalf("install copy: %v", err)
	}

	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyCopy,
		Targets:         []config.Target{tgt},
	}
	entries := sync.Inspect(cfg, []source.Item{item})
	if entries[0].Status != sync.StatusCopied {
		t.Errorf("want StatusCopied, got %q", entries[0].Status)
	}
}

func TestInspect_Drifted(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()
	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# hello")
	tgt := config.Target{Name: "t", Path: tgtDir}

	if _, err := sync.Install(tgt, config.StrategyCopy, item); err != nil {
		t.Fatalf("install copy: %v", err)
	}

	// Modify the source to make the copy drift.
	if err := os.WriteFile(item.Path, []byte("# changed"), 0o644); err != nil {
		t.Fatalf("modify source: %v", err)
	}

	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyCopy,
		Targets:         []config.Target{tgt},
	}
	entries := sync.Inspect(cfg, []source.Item{item})
	if entries[0].Status != sync.StatusDrifted {
		t.Errorf("want StatusDrifted after source change, got %q", entries[0].Status)
	}
}

func TestInspect_NotApplicable(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()
	// Rule items are not supported by the claude agent profile.
	ruleItem := source.Item{Kind: source.KindRule, Name: ".cursorrules", Path: filepath.Join(src, ".cursorrules")}
	if err := os.WriteFile(ruleItem.Path, []byte("rule"), 0o644); err != nil {
		t.Fatalf("write rule: %v", err)
	}

	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyLink,
		Targets:         []config.Target{{Name: "t", Path: tgtDir, Agent: "claude"}},
	}
	entries := sync.Inspect(cfg, []source.Item{ruleItem})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Status != sync.StatusNotApplicable {
		t.Errorf("want StatusNotApplicable for unsupported kind, got %q", entries[0].Status)
	}
}

func TestInspect_ExcludedItemNotInEntries(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()
	item := makeSourceItem(t, src, source.KindContext, "GEMINI.md", "# gemini")

	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyLink,
		Targets: []config.Target{{
			Name:    "t",
			Path:    tgtDir,
			Exclude: []string{"context/GEMINI.md"},
		}},
	}
	entries := sync.Inspect(cfg, []source.Item{item})
	if len(entries) != 0 {
		t.Errorf("excluded item should produce no entries, got %d", len(entries))
	}
}

func TestInspect_Copied_Dir(t *testing.T) {
	// Exercises the sameContent directory branch: both source and dest are dirs.
	src := t.TempDir()
	tgtDir := t.TempDir()

	// Skill items are directories.
	skillPath := filepath.Join(src, "my-skill")
	if err := os.MkdirAll(skillPath, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillPath, "main.md"), []byte("# skill"), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}
	item := source.Item{Kind: source.KindSkill, Name: "my-skill", Path: skillPath}
	tgt := config.Target{Name: "t", Path: tgtDir}

	if _, err := sync.Install(tgt, config.StrategyCopy, item); err != nil {
		t.Fatalf("install copy (dir): %v", err)
	}

	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyCopy,
		Targets:         []config.Target{tgt},
	}
	entries := sync.Inspect(cfg, []source.Item{item})
	if entries[0].Status != sync.StatusCopied {
		t.Errorf("copied dir: want StatusCopied, got %q", entries[0].Status)
	}

	// Add a file to the target copy to create drift.
	if err := os.WriteFile(filepath.Join(tgtDir, "skills", "my-skill", "extra.md"), []byte("extra"), 0o644); err != nil {
		t.Fatalf("add extra file: %v", err)
	}
	entries = sync.Inspect(cfg, []source.Item{item})
	if entries[0].Status != sync.StatusDrifted {
		t.Errorf("drifted dir: want StatusDrifted, got %q", entries[0].Status)
	}
}

func TestInspect_Drifted_DirNestedContent(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()

	skillPath := filepath.Join(src, "my-skill")
	if err := os.MkdirAll(filepath.Join(skillPath, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillPath, "nested", "main.md"), []byte("# skill"), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}
	item := source.Item{Kind: source.KindSkill, Name: "my-skill", Path: skillPath}
	tgt := config.Target{Name: "t", Path: tgtDir}

	if _, err := sync.Install(tgt, config.StrategyCopy, item); err != nil {
		t.Fatalf("install copy (dir): %v", err)
	}
	if err := os.WriteFile(filepath.Join(tgtDir, "skills", "my-skill", "nested", "main.md"), []byte("# changed"), 0o644); err != nil {
		t.Fatalf("modify nested copy: %v", err)
	}

	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyCopy,
		Targets:         []config.Target{tgt},
	}
	entries := sync.Inspect(cfg, []source.Item{item})
	if entries[0].Status != sync.StatusDrifted {
		t.Errorf("nested drift: want StatusDrifted, got %q", entries[0].Status)
	}
}

// ---------------------------------------------------------------------------
// Uninstall
// ---------------------------------------------------------------------------

func TestUninstall_RefusesUnmanaged(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()
	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# hello")
	tgt := config.Target{Name: "t", Path: tgtDir}

	// Write an unmanaged plain file at dest.
	dest := filepath.Join(tgtDir, "CLAUDE.md")
	if err := os.WriteFile(dest, []byte("unmanaged"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := sync.Uninstall(tgt, config.StrategyLink, item)
	if err == nil {
		t.Error("Uninstall should refuse to remove an unmanaged file")
	}

	// File must still exist.
	if _, err := os.Lstat(dest); err != nil {
		t.Error("unmanaged file should not be removed")
	}
}

// ---------------------------------------------------------------------------
// Install
// ---------------------------------------------------------------------------

func TestInstall_LinkStrategy(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()
	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# hello")
	tgt := config.Target{Name: "t", Path: tgtDir}

	st, err := sync.Install(tgt, config.StrategyLink, item)
	if err != nil {
		t.Fatalf("Install link: %v", err)
	}
	if st != sync.StatusLinked {
		t.Errorf("want StatusLinked, got %q", st)
	}
	dest := filepath.Join(tgtDir, "CLAUDE.md")
	fi, err := os.Lstat(dest)
	if err != nil {
		t.Fatalf("dest should exist: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink at dest")
	}
}

func TestInstall_CopyStrategy(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()
	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# hello")
	tgt := config.Target{Name: "t", Path: tgtDir}

	st, err := sync.Install(tgt, config.StrategyCopy, item)
	if err != nil {
		t.Fatalf("Install copy: %v", err)
	}
	if st != sync.StatusCopied {
		t.Errorf("want StatusCopied, got %q", st)
	}
	dest := filepath.Join(tgtDir, "CLAUDE.md")
	fi, err := os.Lstat(dest)
	if err != nil {
		t.Fatalf("dest should exist: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Error("expected real file at dest for copy strategy")
	}
	data, _ := os.ReadFile(dest)
	if string(data) != "# hello" {
		t.Errorf("content mismatch: %q", data)
	}
}

func TestInstall_Idempotent_AlreadyLinked(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()
	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# hello")
	tgt := config.Target{Name: "t", Path: tgtDir}

	if _, err := sync.Install(tgt, config.StrategyLink, item); err != nil {
		t.Fatalf("first install: %v", err)
	}

	st, err := sync.Install(tgt, config.StrategyLink, item)
	if err != nil {
		t.Fatalf("second install should not error: %v", err)
	}
	if st != sync.StatusLinked {
		t.Errorf("second install: want StatusLinked, got %q", st)
	}
}

func TestInstall_Drifted_Reinstalls(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()
	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# original")
	tgt := config.Target{Name: "t", Path: tgtDir}

	if _, err := sync.Install(tgt, config.StrategyCopy, item); err != nil {
		t.Fatalf("first install (copy): %v", err)
	}

	// Modify source to create drift.
	if err := os.WriteFile(item.Path, []byte("# changed"), 0o644); err != nil {
		t.Fatalf("modify source: %v", err)
	}

	// Install again; should replace the drifted copy.
	st, err := sync.Install(tgt, config.StrategyCopy, item)
	if err != nil {
		t.Fatalf("reinstall on drifted: %v", err)
	}
	if st != sync.StatusCopied {
		t.Errorf("reinstall: want StatusCopied, got %q", st)
	}
	dest := filepath.Join(tgtDir, "CLAUDE.md")
	data, _ := os.ReadFile(dest)
	if string(data) != "# changed" {
		t.Errorf("drifted reinstall: dest should have new content, got %q", data)
	}
}

// ---------------------------------------------------------------------------
// Adopt
// ---------------------------------------------------------------------------

func TestAdopt_ReplacesUnmanaged(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()
	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# managed")
	tgt := config.Target{Name: "t", Path: tgtDir}

	// Write an unmanaged file at dest.
	dest := filepath.Join(tgtDir, "CLAUDE.md")
	if err := os.WriteFile(dest, []byte("# unmanaged"), 0o644); err != nil {
		t.Fatalf("write unmanaged: %v", err)
	}

	st, err := sync.Adopt(tgt, config.StrategyLink, item)
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if st != sync.StatusLinked {
		t.Errorf("want StatusLinked after adopt, got %q", st)
	}

	// Dest should now be a symlink pointing at source.
	fi, err := os.Lstat(dest)
	if err != nil {
		t.Fatalf("dest should exist after adopt: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink after adopt")
	}
}

func TestAdopt_AbsentBehavesLikeInstall(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()
	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# managed")
	tgt := config.Target{Name: "t", Path: tgtDir}

	st, err := sync.Adopt(tgt, config.StrategyLink, item)
	if err != nil {
		t.Fatalf("Adopt on absent: %v", err)
	}
	if st != sync.StatusLinked {
		t.Errorf("want StatusLinked, got %q", st)
	}
}

// ---------------------------------------------------------------------------
// ScanTargetDirs
// ---------------------------------------------------------------------------

func TestScanTargetDirs_MatchedAndUnmanaged(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()

	// Source has one hook item.
	srcHook := makeSourceItem(t, src, source.KindHook, "my-hook.sh", "#!/bin/sh")

	// Install it into the target via symlink.
	hooksDir := filepath.Join(tgtDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	if err := os.Symlink(srcHook.Path, filepath.Join(hooksDir, "my-hook.sh")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	// Also place an extra hook not in the source.
	if err := os.WriteFile(filepath.Join(hooksDir, "extra.sh"), []byte("#!/bin/sh"), 0o644); err != nil {
		t.Fatalf("write extra: %v", err)
	}

	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyLink,
		Targets: []config.Target{{
			Name:  "t",
			Path:  tgtDir,
			Agent: "claude",
		}},
	}

	entries := sync.ScanTargetDirs(cfg, []source.Item{srcHook})
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (1 matched + 1 unmanaged), got %d", len(entries))
	}

	byName := make(map[string]sync.Status, 2)
	for _, e := range entries {
		byName[e.Item.Name] = e.Status
	}

	if byName["my-hook.sh"] != sync.StatusLinked {
		t.Errorf("my-hook.sh: want StatusLinked, got %q", byName["my-hook.sh"])
	}
	if byName["extra.sh"] != sync.StatusUnmanaged {
		t.Errorf("extra.sh: want StatusUnmanaged, got %q", byName["extra.sh"])
	}
}

// ---------------------------------------------------------------------------
// Sync
// ---------------------------------------------------------------------------

func TestSync_DryRunMakesNoChanges(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()
	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# hello")
	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyLink,
		Targets:         []config.Target{{Name: "t", Path: tgtDir}},
	}

	results := sync.Sync(cfg, []source.Item{item}, nil, true, false)
	if len(results) != 1 {
		t.Fatalf("dry-run: expected 1 result, got %d", len(results))
	}
	if results[0].OldStatus != sync.StatusAbsent {
		t.Errorf("dry-run OldStatus: want absent, got %q", results[0].OldStatus)
	}

	// No file should have been created.
	dest := filepath.Join(tgtDir, "CLAUDE.md")
	if _, err := os.Lstat(dest); !os.IsNotExist(err) {
		t.Error("dry-run must not write files")
	}
}

func TestSync_RealRunInstallsAndRecordsHash(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()
	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# hello")
	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyLink,
		Targets:         []config.Target{{Name: "t", Path: tgtDir}},
	}

	lck := make(lock.Lock)
	results := sync.Sync(cfg, []source.Item{item}, lck, false, false)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Entry.Status != sync.StatusLinked {
		t.Errorf("want StatusLinked, got %q", results[0].Entry.Status)
	}

	// Symlink must exist.
	dest := filepath.Join(tgtDir, "CLAUDE.md")
	fi, err := os.Lstat(dest)
	if err != nil {
		t.Fatalf("dest should exist: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink at dest")
	}

	// Lock entry must be recorded.
	if len(lck) == 0 {
		t.Error("expected lock entry to be recorded")
	}
	for destPath, entry := range lck {
		if entry.Hash == "" {
			t.Errorf("lock entry at %q has empty hash", destPath)
		}
	}
}

func TestSync_ForceAdoptsUnmanaged(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()
	item := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# managed")

	// Write an unmanaged file at dest.
	dest := filepath.Join(tgtDir, "CLAUDE.md")
	if err := os.WriteFile(dest, []byte("# unmanaged"), 0o644); err != nil {
		t.Fatalf("write unmanaged: %v", err)
	}

	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyLink,
		Targets:         []config.Target{{Name: "t", Path: tgtDir}},
	}

	// Without force: unmanaged item is skipped.
	r := sync.Sync(cfg, []source.Item{item}, nil, false, false)
	if len(r) != 0 {
		t.Errorf("without force, unmanaged should be skipped; got %d results", len(r))
	}

	// With force: unmanaged is adopted.
	r = sync.Sync(cfg, []source.Item{item}, nil, false, true)
	if len(r) != 1 {
		t.Fatalf("with force, expect 1 result; got %d", len(r))
	}
	if r[0].Entry.Status != sync.StatusLinked {
		t.Errorf("force: want StatusLinked, got %q", r[0].Entry.Status)
	}
}

func TestSync_SkipsDisabledAndNA(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()
	ctx := makeSourceItem(t, src, source.KindContext, "CLAUDE.md", "# ctx")
	rule := source.Item{Kind: source.KindRule, Name: ".cursorrules", Path: filepath.Join(src, ".cursorrules")}
	if err := os.WriteFile(rule.Path, []byte("rule"), 0o644); err != nil {
		t.Fatalf("write rule: %v", err)
	}

	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyLink,
		Targets: []config.Target{{
			Name:     "t",
			Path:     tgtDir,
			Agent:    "claude", // claude supports context but not rule
			Disabled: []string{"CLAUDE.md"},
		}},
	}

	results := sync.Sync(cfg, []source.Item{ctx, rule}, nil, false, false)
	if len(results) != 0 {
		t.Errorf("disabled and N/A items should produce no Sync results, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// CopyAny / copyDir
// ---------------------------------------------------------------------------

func TestCopyAny_File(t *testing.T) {
	src := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "dest.txt")
	if err := sync.CopyAny(src, dst); err != nil {
		t.Fatalf("CopyAny file: %v", err)
	}
	data, _ := os.ReadFile(dst)
	if string(data) != "hello" {
		t.Errorf("CopyAny file: content mismatch, got %q", data)
	}
}

func TestCopyAny_Dir(t *testing.T) {
	srcDir := t.TempDir()
	mkfile := func(rel, content string) {
		t.Helper()
		p := filepath.Join(srcDir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	mkfile("README.md", "# hello")
	mkfile("sub/file.txt", "nested")

	dstDir := filepath.Join(t.TempDir(), "dest")
	if err := sync.CopyAny(srcDir, dstDir); err != nil {
		t.Fatalf("CopyAny dir: %v", err)
	}

	for rel, want := range map[string]string{
		"README.md":    "# hello",
		"sub/file.txt": "nested",
	} {
		data, err := os.ReadFile(filepath.Join(dstDir, rel))
		if err != nil {
			t.Errorf("CopyAny dir: %q missing: %v", rel, err)
			continue
		}
		if string(data) != want {
			t.Errorf("CopyAny dir: %q content %q, want %q", rel, data, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Command and rule kind support
// ---------------------------------------------------------------------------

func TestScanTargetDirs_PicksUpCommands(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()

	// Source has one command item.
	cmdPath := filepath.Join(src, "review.md")
	if err := os.WriteFile(cmdPath, []byte("# review"), 0o644); err != nil {
		t.Fatalf("write command: %v", err)
	}
	srcCmd := source.Item{Kind: source.KindCommand, Name: "review.md", Path: cmdPath}

	// Install it into the target under commands/.
	cmdDir := filepath.Join(tgtDir, "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatalf("mkdir commands: %v", err)
	}
	if err := os.Symlink(cmdPath, filepath.Join(cmdDir, "review.md")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyLink,
		Targets: []config.Target{{
			Name:  "claude",
			Path:  tgtDir,
			Agent: "claude",
		}},
	}

	entries := sync.ScanTargetDirs(cfg, []source.Item{srcCmd})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for installed command, got %d", len(entries))
	}
	if entries[0].Status != sync.StatusLinked {
		t.Errorf("want StatusLinked for installed command, got %q", entries[0].Status)
	}
	if entries[0].Item.Kind != source.KindCommand {
		t.Errorf("want KindCommand, got %q", entries[0].Item.Kind)
	}
}

func TestInstall_CommandToClaudeTarget(t *testing.T) {
	src := t.TempDir()
	tgtDir := t.TempDir()

	cmdPath := filepath.Join(src, "deploy.md")
	if err := os.WriteFile(cmdPath, []byte("# deploy"), 0o644); err != nil {
		t.Fatalf("write command: %v", err)
	}
	cmdItem := source.Item{Kind: source.KindCommand, Name: "deploy.md", Path: cmdPath}
	tgt := config.Target{Name: "claude", Path: tgtDir, Agent: "claude"}

	status, err := sync.Install(tgt, config.StrategyLink, cmdItem)
	if err != nil {
		t.Fatalf("Install command: %v", err)
	}
	if status != sync.StatusLinked {
		t.Errorf("want StatusLinked, got %q", status)
	}
	dest := filepath.Join(tgtDir, "commands", "deploy.md")
	if _, err := os.Lstat(dest); err != nil {
		t.Errorf("command not installed at expected path %s: %v", dest, err)
	}
}

func TestInstall_RuleWithDestNameOverride(t *testing.T) {
	// Cline profile overrides the install filename to ".clinerules".
	src := t.TempDir()
	tgtDir := t.TempDir()

	rulePath := filepath.Join(src, "typescript.md")
	if err := os.WriteFile(rulePath, []byte("# ts rules"), 0o644); err != nil {
		t.Fatalf("write rule: %v", err)
	}
	ruleItem := source.Item{Kind: source.KindRule, Name: "typescript.md", Path: rulePath}
	tgt := config.Target{Name: "cline", Path: tgtDir, Agent: "cline"}

	status, err := sync.Install(tgt, config.StrategyLink, ruleItem)
	if err != nil {
		t.Fatalf("Install rule to cline: %v", err)
	}
	if status != sync.StatusLinked {
		t.Errorf("want StatusLinked, got %q", status)
	}
	// Should land at .clinerules (root, renamed by DestName).
	dest := filepath.Join(tgtDir, ".clinerules")
	if _, err := os.Lstat(dest); err != nil {
		t.Errorf("rule not installed as .clinerules at %s: %v", dest, err)
	}
}

func TestInstall_RuleToCursorDirectory(t *testing.T) {
	// Cursor profile uses the directory form: .cursor/rules/
	src := t.TempDir()
	tgtDir := t.TempDir()

	rulePath := filepath.Join(src, "no-console.md")
	if err := os.WriteFile(rulePath, []byte("# no console"), 0o644); err != nil {
		t.Fatalf("write rule: %v", err)
	}
	ruleItem := source.Item{Kind: source.KindRule, Name: "no-console.md", Path: rulePath}
	tgt := config.Target{Name: "cursor", Path: tgtDir, Agent: "cursor"}

	status, err := sync.Install(tgt, config.StrategyLink, ruleItem)
	if err != nil {
		t.Fatalf("Install rule to cursor: %v", err)
	}
	if status != sync.StatusLinked {
		t.Errorf("want StatusLinked, got %q", status)
	}
	dest := filepath.Join(tgtDir, ".cursor", "rules", "no-console.md")
	if _, err := os.Lstat(dest); err != nil {
		t.Errorf("rule not installed at %s: %v", dest, err)
	}
}

func TestCopyAny_SymlinkedRoot(t *testing.T) {
	// Create a real dir with content, then make a symlink to it.
	realDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(realDir, "inner.txt"), []byte("via symlink"), 0o644); err != nil {
		t.Fatalf("write inner: %v", err)
	}

	linkDir := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	// Copy through the symlinked root; EvalSymlinks in copyDir must resolve it.
	dstDir := filepath.Join(t.TempDir(), "dest")
	if err := sync.CopyAny(linkDir, dstDir); err != nil {
		t.Fatalf("CopyAny symlinked root: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dstDir, "inner.txt"))
	if err != nil {
		t.Fatalf("inner.txt not copied through symlinked root: %v", err)
	}
	if string(data) != "via symlink" {
		t.Errorf("content mismatch: %q", data)
	}
}

func TestCopyAny_Dir_ToleratesDanglingNestedSymlink(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "real.txt"), []byte("kept"), 0o644); err != nil {
		t.Fatalf("write real.txt: %v", err)
	}
	// A symlink whose target does not exist, nested inside the tree being
	// copied — must not abort the copy (regression: previously errored via
	// os.Open on the dangling target).
	dangling := filepath.Join(srcDir, "gone")
	if err := os.Symlink(filepath.Join(srcDir, "does-not-exist.txt"), dangling); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	dstDir := filepath.Join(t.TempDir(), "dest")
	if err := sync.CopyAny(srcDir, dstDir); err != nil {
		t.Fatalf("CopyAny with dangling nested symlink: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dstDir, "real.txt"))
	if err != nil || string(data) != "kept" {
		t.Errorf("real.txt not copied correctly: %v %q", err, data)
	}

	fi, err := os.Lstat(filepath.Join(dstDir, "gone"))
	if err != nil {
		t.Fatalf("dangling symlink not recreated: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("expected 'gone' to be recreated as a symlink")
	}
}
