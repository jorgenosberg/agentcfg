package lock_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jorgenosberg/agentcfg/internal/lock"
)

func TestLoadMissingFile(t *testing.T) {
	l, err := lock.Load(filepath.Join(t.TempDir(), "no-such-file.json"))
	if err != nil {
		t.Fatalf("expected nil error for missing lockfile, got %v", err)
	}
	if len(l) != 0 {
		t.Errorf("expected empty lock, got %d entries", len(l))
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "locks.json")
	now := time.Now().UTC().Truncate(time.Second)

	original := lock.Lock{
		"/some/dest/file.md": {Hash: "abc123", InstalledAt: now},
		"/other/dest":        {Hash: "def456", InstalledAt: now.Add(time.Minute)},
	}

	if err := lock.Save(path, original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := lock.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded) != len(original) {
		t.Fatalf("expected %d entries, got %d", len(original), len(loaded))
	}
	for k, want := range original {
		got, ok := loaded[k]
		if !ok {
			t.Errorf("missing key %s", k)
			continue
		}
		if got.Hash != want.Hash {
			t.Errorf("key %s: hash mismatch: want %q got %q", k, want.Hash, got.Hash)
		}
		if !got.InstalledAt.Equal(want.InstalledAt) {
			t.Errorf("key %s: time mismatch: want %v got %v", k, want.InstalledAt, got.InstalledAt)
		}
	}
}

func TestSaveCreatesParentDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "c", "locks.json")
	if err := lock.Save(path, lock.Lock{}); err != nil {
		t.Fatalf("Save should create parent dirs: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file should exist after Save: %v", err)
	}
}

func TestHashPathFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	h1, err := lock.HashPath(path)
	if err != nil {
		t.Fatalf("HashPath: %v", err)
	}
	if h1 == "" {
		t.Fatal("expected non-empty hash")
	}

	// Same content → same hash
	h2, err := lock.HashPath(path)
	if err != nil {
		t.Fatalf("HashPath second call: %v", err)
	}
	if h1 != h2 {
		t.Errorf("hash not stable: %q != %q", h1, h2)
	}
}

func TestHashPathFileChangesOnContentChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("version 1"), 0o644); err != nil {
		t.Fatal(err)
	}

	h1, _ := lock.HashPath(path)
	if err := os.WriteFile(path, []byte("version 2"), 0o644); err != nil {
		t.Fatal(err)
	}
	h2, _ := lock.HashPath(path)
	if h1 == h2 {
		t.Error("hash should change when file content changes")
	}
}

func TestHashPathDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("aaa"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.md"), []byte("bbb"), 0o644); err != nil {
		t.Fatal(err)
	}

	h1, err := lock.HashPath(dir)
	if err != nil {
		t.Fatalf("HashPath dir: %v", err)
	}
	if h1 == "" {
		t.Fatal("expected non-empty hash for directory")
	}

	// Stable across repeated calls
	h2, _ := lock.HashPath(dir)
	if h1 != h2 {
		t.Error("directory hash should be stable")
	}
}

func TestHashPathDirectoryChangesOnEdit(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.md"), []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	h1, _ := lock.HashPath(dir)
	if err := os.WriteFile(filepath.Join(dir, "file.md"), []byte("modified"), 0o644); err != nil {
		t.Fatal(err)
	}
	h2, _ := lock.HashPath(dir)
	if h1 == h2 {
		t.Error("directory hash should change when a file inside changes")
	}
}

func TestHashPathDirectoryChangesOnAddFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}

	h1, _ := lock.HashPath(dir)
	if err := os.WriteFile(filepath.Join(dir, "b.md"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	h2, _ := lock.HashPath(dir)
	if h1 == h2 {
		t.Error("directory hash should change when a new file is added")
	}
}

func TestHashPathNonexistent(t *testing.T) {
	_, err := lock.HashPath(filepath.Join(t.TempDir(), "no-such"))
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}
