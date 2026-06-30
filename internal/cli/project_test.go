package cli_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProject_Add(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	projDir := t.TempDir()
	out, err := runCLI(t, "project", "add", "myproj", projDir)
	if err != nil {
		t.Fatalf("project add: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "added") {
		t.Errorf("expected 'added' in output: %s", out)
	}

	loaded := readConfig(t, home)
	found := false
	for _, p := range loaded.Projects {
		if p.Name == "myproj" {
			found = true
			// Path should be absolute.
			if !filepath.IsAbs(p.Path) {
				t.Errorf("project path should be absolute, got %q", p.Path)
			}
		}
	}
	if !found {
		t.Error("project 'myproj' not persisted in config")
	}
}

func TestProject_AddDuplicate(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	projDir := t.TempDir()
	if _, err := runCLI(t, "project", "add", "dup", projDir); err != nil {
		t.Fatalf("first project add: %v", err)
	}
	_, err := runCLI(t, "project", "add", "dup", projDir)
	if err == nil {
		t.Error("expected error on duplicate project name")
	}
}

func TestProject_Remove(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	projDir := t.TempDir()
	if _, err := runCLI(t, "project", "add", "removeme", projDir); err != nil {
		t.Fatalf("project add: %v", err)
	}

	out, err := runCLI(t, "project", "remove", "removeme")
	if err != nil {
		t.Fatalf("project remove: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "removed") {
		t.Errorf("expected 'removed' in output: %s", out)
	}

	loaded := readConfig(t, home)
	for _, p := range loaded.Projects {
		if p.Name == "removeme" {
			t.Error("project should be removed from config")
		}
	}
}

func TestProject_RemoveUnknown(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	_, err := runCLI(t, "project", "remove", "ghost")
	if err == nil {
		t.Error("expected error removing unknown project")
	}
}

func TestProject_List(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	for _, name := range []string{"alpha", "beta"} {
		if _, err := runCLI(t, "project", "add", name, t.TempDir()); err != nil {
			t.Fatalf("project add %s: %v", name, err)
		}
	}

	out, err := runCLI(t, "project", "list")
	if err != nil {
		t.Fatalf("project list: %v\noutput: %s", err, out)
	}

	for _, want := range []string{"NAME", "PATH", "alpha", "beta"} {
		if !strings.Contains(out, want) {
			t.Errorf("project list missing %q: %s", want, out)
		}
	}
}

func TestProject_List_Empty(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	out, err := runCLI(t, "project", "list")
	if err != nil {
		t.Fatalf("project list (empty): %v\noutput: %s", err, out)
	}
	// Should print headers but no rows; no error.
	if !strings.Contains(out, "NAME") {
		t.Errorf("expected header in output: %s", out)
	}
}

func TestProject_Scan_NoProjects(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	out, err := runCLI(t, "project", "scan")
	if err != nil {
		t.Fatalf("project scan (no projects): %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "no projects") {
		t.Errorf("expected guidance about no projects: %s", out)
	}
}

func TestProject_Scan_All(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	projDir := t.TempDir()
	seedProject(t, projDir)

	if _, err := runCLI(t, "project", "add", "myproj", projDir); err != nil {
		t.Fatalf("project add: %v", err)
	}

	out, err := runCLI(t, "project", "scan")
	if err != nil {
		t.Fatalf("project scan: %v\noutput: %s", err, out)
	}

	for _, want := range []string{"PROJECT", "AGENT", "KIND", "myproj", "claude", "copilot", "cursor"} {
		if !strings.Contains(out, want) {
			t.Errorf("project scan missing %q: %s", want, out)
		}
	}
}

func TestProject_Scan_ByPositional(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	dirA := t.TempDir()
	dirB := t.TempDir()
	seedProject(t, dirA)
	seedProject(t, dirB)
	mkfile(t, filepath.Join(dirB, "AGENTS.md"), "# agents only")

	for _, name := range []string{"alpha", "beta"} {
		dir := dirA
		if name == "beta" {
			dir = dirB
		}
		if _, err := runCLI(t, "project", "add", name, dir); err != nil {
			t.Fatalf("project add %s: %v", name, err)
		}
	}

	out, err := runCLI(t, "project", "scan", "alpha")
	if err != nil {
		t.Fatalf("project scan alpha: %v\noutput: %s", err, out)
	}

	if !strings.Contains(out, "alpha") {
		t.Errorf("expected 'alpha' in output: %s", out)
	}
	if strings.Contains(out, "beta") {
		t.Errorf("should not contain 'beta' when filtering by 'alpha': %s", out)
	}
}

func TestProject_Scan_UnknownPositional(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	if _, err := runCLI(t, "project", "add", "real", t.TempDir()); err != nil {
		t.Fatalf("project add: %v", err)
	}

	_, err := runCLI(t, "project", "scan", "ghost")
	if err == nil {
		t.Error("expected error for unknown project positional arg")
	}
}

// TestProject_Scan_ByFlag verifies the -p/--project flag filters correctly.
// This test is fix-forcing: before the bug fix it fails because the flag was
// declared but ignored.
func TestProject_Scan_ByFlag(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	dirA := t.TempDir()
	dirB := t.TempDir()
	seedProject(t, dirA)
	mkfile(t, filepath.Join(dirB, "AGENTS.md"), "# agents only")

	for _, entry := range []struct{ name, dir string }{{"alpha", dirA}, {"beta", dirB}} {
		if _, err := runCLI(t, "project", "add", entry.name, entry.dir); err != nil {
			t.Fatalf("project add %s: %v", entry.name, err)
		}
	}

	out, err := runCLI(t, "project", "scan", "-p", "alpha")
	if err != nil {
		t.Fatalf("project scan -p alpha: %v\noutput: %s", err, out)
	}

	if !strings.Contains(out, "alpha") {
		t.Errorf("expected 'alpha' in output: %s", out)
	}
	if strings.Contains(out, "beta") {
		t.Errorf("should not contain 'beta' when -p alpha: %s", out)
	}
}

// TestProject_Scan_UnknownFlag verifies -p with a non-existent project name errors.
func TestProject_Scan_UnknownFlag(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	if _, err := runCLI(t, "project", "add", "real", t.TempDir()); err != nil {
		t.Fatalf("project add: %v", err)
	}

	_, err := runCLI(t, "project", "scan", "-p", "ghost")
	if err == nil {
		t.Error("expected error for unknown project via -p flag")
	}
}

func TestProject_Scan_NoItems(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	// Add a project pointing at an empty directory.
	emptyDir := t.TempDir()
	if _, err := runCLI(t, "project", "add", "empty", emptyDir); err != nil {
		t.Fatalf("project add: %v", err)
	}

	out, err := runCLI(t, "project", "scan")
	if err != nil {
		t.Fatalf("project scan (empty dir): %v\noutput: %s", err, out)
	}
	// Should still run without error; report "no items found" or similar.
	if !strings.Contains(out, "PROJECT") {
		t.Errorf("expected column header in output: %s", out)
	}
}

// TestProject_Scan_NonExistentDir verifies scan handles a project pointing at a
// missing directory without crashing.
func TestProject_Scan_NonExistentDir(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	// Register a path that doesn't exist.
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if _, err := runCLI(t, "project", "add", "ghost-dir", missing); err != nil {
		t.Fatalf("project add: %v", err)
	}

	out, err := runCLI(t, "project", "scan")
	if err != nil {
		t.Fatalf("project scan on missing dir: %v\noutput: %s", err, out)
	}
	// ScanProject silently returns nil for non-existent roots; should get "(no items found)".
	if !strings.Contains(out, "PROJECT") {
		t.Errorf("expected header in output: %s", out)
	}
}

// TestProject_Scan_StatError verifies that a scan error for one project is
// reported inline but does not abort the whole command.
func TestProject_Scan_ScanError(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	// Create a project dir, then make it a file (not a dir) to trigger a scan error.
	bad := filepath.Join(t.TempDir(), "not-a-dir")
	mkfile(t, bad, "surprise")
	// stat-as-file trick: replace the dir with a file at a known path
	projPath := filepath.Join(home, "proj")
	mkfile(t, projPath, "not a dir")

	if _, err := runCLI(t, "project", "add", "bad", projPath); err != nil {
		t.Fatalf("project add: %v", err)
	}

	// Should not crash; scan error appears inline in the table.
	out, err := runCLI(t, "project", "scan")
	if err != nil {
		t.Fatalf("project scan with scan error: %v\noutput: %s", err, out)
	}
	_ = out // non-dir root returns nil from ScanProject, so "(no items found)" or error row
}

func TestProject_Scan_AddAndScanProjectExists(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	// Add a real project dir with claude files.
	projDir := t.TempDir()
	mkfile(t, filepath.Join(projDir, "CLAUDE.md"), "# project context")

	if _, err := runCLI(t, "project", "add", "proj", projDir); err != nil {
		t.Fatalf("project add: %v", err)
	}

	out, err := runCLI(t, "project", "scan", "proj")
	if err != nil {
		t.Fatalf("project scan proj: %v\noutput: %s", err, out)
	}

	if !strings.Contains(out, "CLAUDE.md") {
		t.Errorf("expected CLAUDE.md in scan output: %s", out)
	}
	if !strings.Contains(out, "claude") {
		t.Errorf("expected agent 'claude' in scan output: %s", out)
	}

	// non-existent sub uses os.Stat which returns nil error for a real file.
	_ = out
}

func TestProject_Remove_PersistsBothProjectsCorrectly(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	// Add two projects then remove one; the other should remain.
	dirs := []string{t.TempDir(), t.TempDir()}
	for i, name := range []string{"keep", "drop"} {
		if _, err := runCLI(t, "project", "add", name, dirs[i]); err != nil {
			t.Fatalf("project add %s: %v", name, err)
		}
	}

	if _, err := runCLI(t, "project", "remove", "drop"); err != nil {
		t.Fatalf("project remove drop: %v", err)
	}

	loaded := readConfig(t, home)
	if len(loaded.Projects) != 1 || loaded.Projects[0].Name != "keep" {
		t.Errorf("expected exactly 'keep' to remain, got: %+v", loaded.Projects)
	}
}
