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
