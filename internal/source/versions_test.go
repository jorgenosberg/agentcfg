package source_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jorgenosberg/agentcfg/internal/source"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestVersionsDirPath(t *testing.T) {
	got := source.VersionsDir("/source/hooks/pre-commit.sh")
	want := "/source/hooks/.versions/pre-commit.sh"
	if got != want {
		t.Errorf("VersionsDir: want %q got %q", want, got)
	}
}

func TestListVersionsEmpty(t *testing.T) {
	dir := t.TempDir()
	item := filepath.Join(dir, "CLAUDE.md")
	writeFile(t, item, "hello")

	versions, err := source.ListVersions(item)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if versions != nil {
		t.Errorf("expected nil versions, got %v", versions)
	}
}

func TestSaveAndListVersions(t *testing.T) {
	dir := t.TempDir()
	item := filepath.Join(dir, "CLAUDE.md")
	writeFile(t, item, "version a")

	if err := source.SaveVersion(item, "v1"); err != nil {
		t.Fatalf("SaveVersion v1: %v", err)
	}
	writeFile(t, item, "version b")
	if err := source.SaveVersion(item, "v2"); err != nil {
		t.Fatalf("SaveVersion v2: %v", err)
	}

	versions, err := source.ListVersions(item)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d: %v", len(versions), versions)
	}
	if versions[0] != "v1" || versions[1] != "v2" {
		t.Errorf("unexpected versions: %v", versions)
	}
}

func TestSaveVersionPreservesContent(t *testing.T) {
	dir := t.TempDir()
	item := filepath.Join(dir, "hook.sh")
	writeFile(t, item, "original content")

	if err := source.SaveVersion(item, "backup"); err != nil {
		t.Fatalf("SaveVersion: %v", err)
	}

	versionPath := filepath.Join(source.VersionsDir(item), "backup")
	if got := readFile(t, versionPath); got != "original content" {
		t.Errorf("version content: want %q got %q", "original content", got)
	}
}

func TestSwitchVersion(t *testing.T) {
	dir := t.TempDir()
	item := filepath.Join(dir, "CLAUDE.md")
	writeFile(t, item, "v1 content")

	if err := source.SaveVersion(item, "v1"); err != nil {
		t.Fatalf("SaveVersion: %v", err)
	}
	writeFile(t, item, "v2 content")
	if err := source.SaveVersion(item, "v2"); err != nil {
		t.Fatalf("SaveVersion: %v", err)
	}

	// Switch back to v1
	if err := source.SwitchVersion(item, "v1"); err != nil {
		t.Fatalf("SwitchVersion: %v", err)
	}
	if got := readFile(t, item); got != "v1 content" {
		t.Errorf("active content after switch: want %q got %q", "v1 content", got)
	}
}

func TestSwitchVersionAutoSavesPrevious(t *testing.T) {
	dir := t.TempDir()
	item := filepath.Join(dir, "CLAUDE.md")
	writeFile(t, item, "original")

	if err := source.SaveVersion(item, "v1"); err != nil {
		t.Fatalf("SaveVersion: %v", err)
	}
	writeFile(t, item, "modified")

	if err := source.SwitchVersion(item, "v1"); err != nil {
		t.Fatalf("SwitchVersion: %v", err)
	}

	// Active should now be v1 content.
	if got := readFile(t, item); got != "original" {
		t.Errorf("active after switch: want %q got %q", "original", got)
	}

	// "previous" should hold what was there before the switch.
	versions, _ := source.ListVersions(item)
	found := false
	for _, v := range versions {
		if v == "previous" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a %q version to be auto-saved, got %v", "previous", versions)
	}
	prevPath := filepath.Join(source.VersionsDir(item), "previous")
	if got := readFile(t, prevPath); got != "modified" {
		t.Errorf("previous version content: want %q got %q", "modified", got)
	}
}

func TestSwitchVersionNotFound(t *testing.T) {
	dir := t.TempDir()
	item := filepath.Join(dir, "CLAUDE.md")
	writeFile(t, item, "content")

	if err := source.SwitchVersion(item, "nonexistent"); err == nil {
		t.Error("expected error for nonexistent version")
	}
}

func TestDeleteVersion(t *testing.T) {
	dir := t.TempDir()
	item := filepath.Join(dir, "hook.sh")
	writeFile(t, item, "content")

	if err := source.SaveVersion(item, "v1"); err != nil {
		t.Fatalf("SaveVersion: %v", err)
	}
	if err := source.DeleteVersion(item, "v1"); err != nil {
		t.Fatalf("DeleteVersion: %v", err)
	}

	versions, _ := source.ListVersions(item)
	if len(versions) != 0 {
		t.Errorf("expected 0 versions after delete, got %d", len(versions))
	}
}

func TestDeleteVersionNotFound(t *testing.T) {
	dir := t.TempDir()
	item := filepath.Join(dir, "hook.sh")
	writeFile(t, item, "content")

	if err := source.DeleteVersion(item, "ghost"); err == nil {
		t.Error("expected error deleting nonexistent version")
	}
}

func TestSaveVersionOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	item := filepath.Join(dir, "CLAUDE.md")
	writeFile(t, item, "first")

	if err := source.SaveVersion(item, "v1"); err != nil {
		t.Fatalf("first SaveVersion: %v", err)
	}
	writeFile(t, item, "second")
	if err := source.SaveVersion(item, "v1"); err != nil {
		t.Fatalf("second SaveVersion: %v", err)
	}

	versionPath := filepath.Join(source.VersionsDir(item), "v1")
	if got := readFile(t, versionPath); got != "second" {
		t.Errorf("overwritten version content: want %q got %q", "second", got)
	}
}

func TestVersionsDirSkippedByScan(t *testing.T) {
	root := t.TempDir()
	// Write a context file and save a version of it
	ctx := filepath.Join(root, "CLAUDE.md")
	writeFile(t, ctx, "context")
	if err := source.SaveVersion(ctx, "v1"); err != nil {
		t.Fatalf("SaveVersion: %v", err)
	}

	items, err := source.Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	for _, it := range items {
		if it.Name == ".versions" {
			t.Error("Scan should skip the .versions directory")
		}
	}
}

func TestSaveVersionDirectory(t *testing.T) {
	root := t.TempDir()
	// Create a skill directory
	skillDir := filepath.Join(root, "my-skill")
	writeFile(t, filepath.Join(skillDir, "README.md"), "skill content")
	writeFile(t, filepath.Join(skillDir, "impl.sh"), "#!/bin/sh\necho hi")

	if err := source.SaveVersion(skillDir, "v1"); err != nil {
		t.Fatalf("SaveVersion dir: %v", err)
	}

	versionPath := filepath.Join(source.VersionsDir(skillDir), "v1")
	data, err := os.ReadFile(filepath.Join(versionPath, "README.md"))
	if err != nil {
		t.Fatalf("read versioned file: %v", err)
	}
	if string(data) != "skill content" {
		t.Errorf("versioned dir content: want %q got %q", "skill content", string(data))
	}
}
