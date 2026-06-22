package backup_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jorgenosberg/agentcfg/internal/backup"
	"github.com/jorgenosberg/agentcfg/internal/config"
)

// mkfile writes content to a file, creating parent dirs.
func mkfile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdirall: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestCreateSnapshot(t *testing.T) {
	targetDir := t.TempDir()
	mkfile(t, filepath.Join(targetDir, "CLAUDE.md"), "# hello")
	mkfile(t, filepath.Join(targetDir, "hooks", "post-commit"), "#!/bin/sh")

	cfg := config.Config{
		Source: t.TempDir(),
		Targets: []config.Target{
			{Name: "claude", Path: targetDir},
		},
	}

	snapRoot := t.TempDir()
	snapDir, err := backup.Create(cfg, snapRoot)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Snapshot directory must exist
	if _, err := os.Stat(snapDir); err != nil {
		t.Fatalf("snapshot dir should exist: %v", err)
	}

	// manifest.json must be present
	manifest := filepath.Join(snapDir, "manifest.json")
	if _, err := os.Stat(manifest); err != nil {
		t.Fatalf("manifest.json should exist: %v", err)
	}

	// Files should have been copied
	if _, err := os.Stat(filepath.Join(snapDir, "claude", "CLAUDE.md")); err != nil {
		t.Errorf("CLAUDE.md should be in snapshot: %v", err)
	}
	if _, err := os.Stat(filepath.Join(snapDir, "claude", "hooks", "post-commit")); err != nil {
		t.Errorf("hooks/post-commit should be in snapshot: %v", err)
	}
}

func TestCreateSkipsMissingTargets(t *testing.T) {
	cfg := config.Config{
		Source: t.TempDir(),
		Targets: []config.Target{
			{Name: "missing", Path: "/no/such/path/ever"},
		},
	}

	snapRoot := t.TempDir()
	snapDir, err := backup.Create(cfg, snapRoot)
	if err != nil {
		t.Fatalf("Create should succeed even with missing target: %v", err)
	}

	// Manifest should exist, just with no target entries
	manifest := filepath.Join(snapDir, "manifest.json")
	if _, err := os.Stat(manifest); err != nil {
		t.Fatalf("manifest.json should still be created: %v", err)
	}
}

func TestCreateMultipleTargets(t *testing.T) {
	t1 := t.TempDir()
	t2 := t.TempDir()
	mkfile(t, filepath.Join(t1, "file1.md"), "t1")
	mkfile(t, filepath.Join(t2, "file2.md"), "t2")

	cfg := config.Config{
		Source: t.TempDir(),
		Targets: []config.Target{
			{Name: "target1", Path: t1},
			{Name: "target2", Path: t2},
		},
	}

	snapRoot := t.TempDir()
	snapDir, err := backup.Create(cfg, snapRoot)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := os.Stat(filepath.Join(snapDir, "target1", "file1.md")); err != nil {
		t.Errorf("target1/file1.md not in snapshot: %v", err)
	}
	if _, err := os.Stat(filepath.Join(snapDir, "target2", "file2.md")); err != nil {
		t.Errorf("target2/file2.md not in snapshot: %v", err)
	}
}

func TestListEmpty(t *testing.T) {
	snaps, err := backup.List(t.TempDir())
	if err != nil {
		t.Fatalf("List on empty dir: %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(snaps))
	}
}

func TestListNonexistentRoot(t *testing.T) {
	snaps, err := backup.List(filepath.Join(t.TempDir(), "no-such"))
	if err != nil {
		t.Fatalf("List on nonexistent dir should not error: %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(snaps))
	}
}

func TestListNewestFirst(t *testing.T) {
	targetDir := t.TempDir()
	mkfile(t, filepath.Join(targetDir, "file.md"), "content")

	cfg := config.Config{
		Source: t.TempDir(),
		Targets: []config.Target{
			{Name: "t", Path: targetDir},
		},
	}

	snapRoot := t.TempDir()

	// Create two snapshots; sleep a second so directory names differ.
	if _, err := backup.Create(cfg, snapRoot); err != nil {
		t.Fatalf("Create 1: %v", err)
	}
	time.Sleep(time.Second)
	if _, err := backup.Create(cfg, snapRoot); err != nil {
		t.Fatalf("Create 2: %v", err)
	}

	snaps, err := backup.List(snapRoot)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snaps))
	}
	if !snaps[0].Timestamp.After(snaps[1].Timestamp) {
		t.Errorf("snapshots should be sorted newest first: %v, %v", snaps[0].Timestamp, snaps[1].Timestamp)
	}
}

func TestListSkipsCorruptManifest(t *testing.T) {
	snapRoot := t.TempDir()

	// Write a corrupt snapshot dir
	badDir := filepath.Join(snapRoot, "20240101-000000")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mkfile(t, filepath.Join(badDir, "manifest.json"), "not valid json{{")

	snaps, err := backup.List(snapRoot)
	if err != nil {
		t.Fatalf("List should not error on corrupt manifest: %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("corrupt snapshot should be skipped, got %d", len(snaps))
	}
}

func TestRestore(t *testing.T) {
	targetDir := t.TempDir()
	mkfile(t, filepath.Join(targetDir, "CLAUDE.md"), "original")
	mkfile(t, filepath.Join(targetDir, "hooks", "hook.sh"), "#!/bin/sh\noriginal")

	cfg := config.Config{
		Source: t.TempDir(),
		Targets: []config.Target{
			{Name: "claude", Path: targetDir},
		},
	}

	snapRoot := t.TempDir()
	snapDir, err := backup.Create(cfg, snapRoot)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Mutate the target after the backup
	mkfile(t, filepath.Join(targetDir, "CLAUDE.md"), "mutated")
	mkfile(t, filepath.Join(targetDir, "new-file.md"), "should disappear after restore")

	if err := backup.Restore(snapDir, cfg); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// CLAUDE.md should be back to "original"
	data, err := os.ReadFile(filepath.Join(targetDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read after restore: %v", err)
	}
	if string(data) != "original" {
		t.Errorf("restore content mismatch: want %q got %q", "original", string(data))
	}

	// hook should still be there
	if _, err := os.Stat(filepath.Join(targetDir, "hooks", "hook.sh")); err != nil {
		t.Errorf("hook.sh should be present after restore: %v", err)
	}
}

func TestRestoreMissingManifest(t *testing.T) {
	if err := backup.Restore(t.TempDir(), config.Config{}); err == nil {
		t.Error("expected error when manifest.json is missing")
	}
}

func TestPruneKeepsNewest(t *testing.T) {
	root := t.TempDir()
	targetDir := t.TempDir()

	// Create 4 snapshot dirs with fake manifests at distinct timestamps.
	for i := range 4 {
		ts := time.Date(2024, 1, i+1, 0, 0, 0, 0, time.UTC)
		dir := filepath.Join(root, ts.Format("20060102-150405"))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		snap := backup.Snapshot{
			Timestamp: ts,
			Targets:   []backup.TargetSnapshot{{Name: "agent", Path: targetDir, Dir: "agent"}},
		}
		data, err := json.Marshal(snap)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	snaps, _ := backup.List(root)
	if len(snaps) != 4 {
		t.Fatalf("expected 4 snapshots before prune, got %d", len(snaps))
	}

	if err := backup.Prune(root, 2); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	after, _ := backup.List(root)
	if len(after) != 2 {
		t.Fatalf("expected 2 snapshots after prune, got %d", len(after))
	}
	if !after[0].Timestamp.After(after[1].Timestamp) {
		t.Error("expected newest-first order after prune")
	}
}

func TestPruneNoOp(t *testing.T) {
	root := t.TempDir()
	if err := backup.Prune(root, 5); err != nil {
		t.Fatalf("Prune on empty dir: %v", err)
	}
}
