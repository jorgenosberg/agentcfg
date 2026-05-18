// internal/sync/sync_test.go
package sync_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jorgenosberg/agentcfg/internal/config"
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
