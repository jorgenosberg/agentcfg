package cli_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jorgenosberg/agentcfg/internal/cli"
	"github.com/jorgenosberg/agentcfg/internal/config"
)

// --- list ---

func TestList_ShowsItems(t *testing.T) {
	home := sandbox(t)
	srcDir := defaultSource(home)

	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)
	addContextItem(t, srcDir, "CLAUDE.md", "# hello")

	out, err := runCLI(t, "list")
	if err != nil {
		t.Fatalf("list: %v\noutput: %s", err, out)
	}
	for _, want := range []string{"KIND", "NAME", "PATH", "CLAUDE.md"} {
		if !strings.Contains(out, want) {
			t.Errorf("list output missing %q: %s", want, out)
		}
	}
}

func TestList_JSON(t *testing.T) {
	home := sandbox(t)
	srcDir := defaultSource(home)

	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)
	addContextItem(t, srcDir, "CLAUDE.md", "# hello")

	out, err := runCLI(t, "list", "--json")
	if err != nil {
		t.Fatalf("list --json: %v\noutput: %s", err, out)
	}
	var items []struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		t.Fatalf("list --json produced invalid JSON: %v\noutput: %s", err, out)
	}
	if len(items) != 1 || items[0].Name != "CLAUDE.md" || items[0].Kind != "context" {
		t.Errorf("unexpected items: %+v", items)
	}
}

func TestList_JSONEmpty(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	out, err := runCLI(t, "list", "--json")
	if err != nil {
		t.Fatalf("list --json (empty): %v\noutput: %s", err, out)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Errorf("expected empty JSON array, got: %s", out)
	}
}

func TestList_JSONError(t *testing.T) {
	sandbox(t) // no config seeded: load must fail

	out, err := runCLI(t, "list", "--json")
	if err == nil {
		t.Fatalf("expected error, got none\noutput: %s", out)
	}
	if !errors.Is(err, cli.ErrSilent) {
		t.Errorf("expected ErrSilent, got: %v", err)
	}
	var payload struct {
		Error string `json:"error"`
	}
	if jerr := json.Unmarshal([]byte(out), &payload); jerr != nil {
		t.Fatalf("stderr is not JSON: %v\noutput: %s", jerr, out)
	}
	if !strings.Contains(payload.Error, "no config found") {
		t.Errorf("unexpected error message: %q", payload.Error)
	}
}

func TestToggle_ExactNameBeatsSharedAlias(t *testing.T) {
	home := sandbox(t)
	srcDir := defaultSource(home)

	cfg := defaultConfig(home)
	cfg.Targets = []config.Target{
		{Name: "claude", Path: filepath.Join(home, ".claude"), Agent: "claude", Alias: "claude"},
		{Name: "claude-knowit", Path: filepath.Join(home, ".claude-knowit"), Agent: "claude", Alias: "claude"},
	}
	seedConfig(t, home, cfg)
	addContextItem(t, srcDir, "CLAUDE.md", "# hello")

	out, err := runCLI(t, "toggle", "CLAUDE.md", "-t", "claude", "--off")
	if err != nil {
		t.Fatalf("toggle: %v\noutput: %s", err, out)
	}

	loaded, err := config.Load(filepath.Join(home, ".agentcfg", "config.json"))
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	for _, tgt := range loaded.Targets {
		disabled := len(tgt.Disabled) > 0
		switch tgt.Name {
		case "claude":
			if !disabled {
				t.Errorf("target %q: expected item disabled", tgt.Name)
			}
		case "claude-knowit":
			if disabled {
				t.Errorf("target %q: alias must not widen exact-name selection, disabled=%v", tgt.Name, tgt.Disabled)
			}
		}
	}
}

func TestToggle_AliasSelectsGroup(t *testing.T) {
	home := sandbox(t)
	srcDir := defaultSource(home)

	cfg := defaultConfig(home)
	cfg.Targets = []config.Target{
		{Name: "claude-personal", Path: filepath.Join(home, ".claude"), Agent: "claude", Alias: "claude"},
		{Name: "claude-knowit", Path: filepath.Join(home, ".claude-knowit"), Agent: "claude", Alias: "claude"},
	}
	seedConfig(t, home, cfg)
	addContextItem(t, srcDir, "CLAUDE.md", "# hello")

	out, err := runCLI(t, "toggle", "CLAUDE.md", "-t", "claude", "--off")
	if err != nil {
		t.Fatalf("toggle: %v\noutput: %s", err, out)
	}

	loaded, err := config.Load(filepath.Join(home, ".agentcfg", "config.json"))
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	for _, tgt := range loaded.Targets {
		if len(tgt.Disabled) == 0 {
			t.Errorf("target %q: expected item disabled via group alias", tgt.Name)
		}
	}
}

func TestStatus_JSON(t *testing.T) {
	home := sandbox(t)
	srcDir := defaultSource(home)

	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)
	addContextItem(t, srcDir, "CLAUDE.md", "# hello")

	out, err := runCLI(t, "status", "--json")
	if err != nil {
		t.Fatalf("status --json: %v\noutput: %s", err, out)
	}
	var entries []struct {
		Target string `json:"target"`
		Kind   string `json:"kind"`
		Item   string `json:"item"`
		Status string `json:"status"`
		Path   string `json:"path"`
		Dest   string `json:"dest"`
	}
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("status --json produced invalid JSON: %v\noutput: %s", err, out)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %+v", len(entries), entries)
	}
	e := entries[0]
	if e.Target != "claude" || e.Item != "CLAUDE.md" || e.Status != "absent" || e.Dest == "" || e.Path == "" {
		t.Errorf("unexpected entry: %+v", e)
	}
}

func TestList_Empty(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	out, err := runCLI(t, "list")
	if err != nil {
		t.Fatalf("list (empty): %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "no items in source") {
		t.Errorf("expected empty-state message in output: %s", out)
	}
}

func TestInstall_ReturnsErrorWhenAnyTargetFails(t *testing.T) {
	home := sandbox(t)
	srcDir := defaultSource(home)

	cfg := defaultConfig(home)
	cfg.Targets[0].Name = "ok"
	bad := cfg.Targets[0]
	bad.Name = "bad"
	bad.Path = filepath.Join(home, ".bad-claude")
	cfg.Targets = append(cfg.Targets, bad)
	seedConfig(t, home, cfg)
	addContextItem(t, srcDir, "CLAUDE.md", "# hello")

	mkfile(t, filepath.Join(bad.Path, "CLAUDE.md"), "# unmanaged")

	out, err := runCLI(t, "install", "CLAUDE.md")
	if err == nil {
		t.Fatalf("expected install to return an error when one target fails; output: %s", out)
	}
	if !strings.Contains(out, "ok") || !strings.Contains(out, "bad") || !strings.Contains(out, "error:") {
		t.Errorf("expected mixed success/error output, got: %s", out)
	}
}

func TestSync_ReturnsErrorWhenAnyInstallFails(t *testing.T) {
	home := sandbox(t)
	srcDir := defaultSource(home)
	readOnly := filepath.Join(home, "readonly")
	if err := os.MkdirAll(readOnly, 0o555); err != nil {
		t.Fatalf("mkdir readonly: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(readOnly, 0o755)
	})

	cfg := defaultConfig(home)
	cfg.Targets[0].Path = filepath.Join(readOnly, "target")
	seedConfig(t, home, cfg)
	addContextItem(t, srcDir, "CLAUDE.md", "# hello")

	out, err := runCLI(t, "sync", "--no-backup")
	if err == nil {
		t.Fatalf("expected sync to return an error when install fails; output: %s", out)
	}
	if !strings.Contains(out, "error:") {
		t.Errorf("expected sync error output, got: %s", out)
	}
}

func TestSync_NoTargetsReportsActionableMessage(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	cfg.Targets = nil
	seedConfig(t, home, cfg)

	out, err := runCLI(t, "sync", "--dry-run")
	if err != nil {
		t.Fatalf("sync --dry-run with no targets: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "no targets configured") {
		t.Errorf("expected no-targets message, got: %s", out)
	}
	if strings.Contains(out, "everything up to date") {
		t.Errorf("no-targets sync should not say everything is up to date: %s", out)
	}
}

// --- toggle ---

func TestToggle_InfersDisable(t *testing.T) {
	home := sandbox(t)
	srcDir := defaultSource(home)

	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)
	addContextItem(t, srcDir, "CLAUDE.md", "# hello")

	// Install first.
	if _, err := runCLI(t, "install", "CLAUDE.md"); err != nil {
		t.Fatalf("install: %v", err)
	}

	// Item is currently enabled; toggle should disable it.
	out, err := runCLI(t, "toggle", "CLAUDE.md")
	if err != nil {
		t.Fatalf("toggle (disable): %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "disabled") {
		t.Errorf("expected 'disabled' in output: %s", out)
	}

	// Symlink should be removed.
	dest := filepath.Join(home, ".claude", "CLAUDE.md")
	if _, err := os.Lstat(dest); !os.IsNotExist(err) {
		t.Error("expected dest to be removed after toggle disable")
	}

	// Disabled list should contain the item.
	loaded := readConfig(t, home)
	found := false
	for _, d := range loaded.Targets[0].Disabled {
		if d == "CLAUDE.md" {
			found = true
		}
	}
	if !found {
		t.Error("CLAUDE.md should be in Disabled after toggle")
	}
}

func TestToggle_InfersEnable(t *testing.T) {
	home := sandbox(t)
	srcDir := defaultSource(home)

	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)
	addContextItem(t, srcDir, "CLAUDE.md", "# hello")

	// Install, disable, then re-enable.
	if _, err := runCLI(t, "install", "CLAUDE.md"); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := runCLI(t, "toggle", "CLAUDE.md"); err != nil {
		t.Fatalf("toggle disable: %v", err)
	}

	// Item is now disabled everywhere; toggle should enable it.
	out, err := runCLI(t, "toggle", "CLAUDE.md")
	if err != nil {
		t.Fatalf("toggle (enable): %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "enabled") {
		t.Errorf("expected 'enabled' in output: %s", out)
	}

	// Symlink should be back.
	dest := filepath.Join(home, ".claude", "CLAUDE.md")
	fi, err := os.Lstat(dest)
	if err != nil {
		t.Fatalf("expected dest to exist after enable: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink at dest after enable")
	}

	// Disabled list should no longer contain the item.
	loaded := readConfig(t, home)
	for _, d := range loaded.Targets[0].Disabled {
		if d == "CLAUDE.md" {
			t.Error("CLAUDE.md should not be in Disabled after enable")
		}
	}
}

func TestToggle_ForceOff(t *testing.T) {
	home := sandbox(t)
	srcDir := defaultSource(home)

	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)
	addContextItem(t, srcDir, "CLAUDE.md", "# hello")

	if _, err := runCLI(t, "install", "CLAUDE.md"); err != nil {
		t.Fatalf("install: %v", err)
	}

	out, err := runCLI(t, "toggle", "--off", "CLAUDE.md")
	if err != nil {
		t.Fatalf("toggle --off: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "disabled") {
		t.Errorf("expected 'disabled' in output: %s", out)
	}
}

func TestToggle_ForceOn(t *testing.T) {
	home := sandbox(t)
	srcDir := defaultSource(home)

	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)
	addContextItem(t, srcDir, "CLAUDE.md", "# hello")

	if _, err := runCLI(t, "install", "CLAUDE.md"); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := runCLI(t, "toggle", "--off", "CLAUDE.md"); err != nil {
		t.Fatalf("toggle --off: %v", err)
	}

	out, err := runCLI(t, "toggle", "--on", "CLAUDE.md")
	if err != nil {
		t.Fatalf("toggle --on: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "enabled") {
		t.Errorf("expected 'enabled' in output: %s", out)
	}
}

func TestToggle_UnknownItem(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	_, err := runCLI(t, "toggle", "ghost-item")
	if err == nil {
		t.Error("expected error for unknown item")
	}
}

// --- unmanage ---

func TestUnmanage_ConvertsSymlinkToFile(t *testing.T) {
	home := sandbox(t)
	srcDir := defaultSource(home)

	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)
	addContextItem(t, srcDir, "CLAUDE.md", "# unmanage test")

	if _, err := runCLI(t, "install", "CLAUDE.md"); err != nil {
		t.Fatalf("install: %v", err)
	}

	// Verify it's a symlink first.
	dest := filepath.Join(home, ".claude", "CLAUDE.md")
	fi, _ := os.Lstat(dest)
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("expected symlink before unmanage")
	}

	out, err := runCLI(t, "unmanage", "CLAUDE.md")
	if err != nil {
		t.Fatalf("unmanage: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "unmanaged") {
		t.Errorf("expected 'unmanaged' in output: %s", out)
	}

	// Dest should now be a real file (not a symlink).
	fi, err = os.Lstat(dest)
	if err != nil {
		t.Fatalf("dest should still exist: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Error("dest should be a real file after unmanage, not a symlink")
	}

	// Content should match the source.
	data, _ := os.ReadFile(dest)
	if !strings.Contains(string(data), "unmanage test") {
		t.Errorf("content mismatch after unmanage: %q", data)
	}

	// Item should be added to Disabled.
	loaded := readConfig(t, home)
	found := false
	for _, d := range loaded.Targets[0].Disabled {
		if d == "CLAUDE.md" {
			found = true
		}
	}
	if !found {
		t.Error("CLAUDE.md should be added to Disabled after unmanage")
	}
}

func TestUnmanage_UnknownItem(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	_, err := runCLI(t, "unmanage", "ghost-item")
	if err == nil {
		t.Error("expected error for unknown item")
	}
}

// --- version ---

func addSkillItem(t *testing.T, srcDir, name string) {
	t.Helper()
	skillDir := filepath.Join(srcDir, "skills", name)
	mkfile(t, filepath.Join(skillDir, "README.md"), "# "+name)
}

func TestVersion_Roundtrip(t *testing.T) {
	home := sandbox(t)
	srcDir := defaultSource(home)

	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)
	addSkillItem(t, srcDir, "my-skill")

	// save
	out, err := runCLI(t, "version", "save", "my-skill", "v1")
	if err != nil {
		t.Fatalf("version save: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "v1") {
		t.Errorf("expected 'v1' in save output: %s", out)
	}

	// list
	out, err = runCLI(t, "version", "list", "my-skill")
	if err != nil {
		t.Fatalf("version list: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "v1") {
		t.Errorf("expected 'v1' in list output: %s", out)
	}

	// switch — SwitchVersion auto-saves current state as "previous" before swapping.
	out, err = runCLI(t, "version", "switch", "my-skill", "v1")
	if err != nil {
		t.Fatalf("version switch: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "v1") {
		t.Errorf("expected 'v1' in switch output: %s", out)
	}

	// delete v1
	out, err = runCLI(t, "version", "delete", "my-skill", "v1")
	if err != nil {
		t.Fatalf("version delete v1: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "v1") {
		t.Errorf("expected 'v1' in delete output: %s", out)
	}

	// delete the auto-created "previous" snapshot too.
	out, err = runCLI(t, "version", "delete", "my-skill", "previous")
	if err != nil {
		t.Fatalf("version delete previous: %v\noutput: %s", err, out)
	}

	// list after all deletes: no saved versions
	out, err = runCLI(t, "version", "list", "my-skill")
	if err != nil {
		t.Fatalf("version list after delete: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "no saved versions") {
		t.Errorf("expected 'no saved versions' after delete: %s", out)
	}
}

func TestVersion_ListEmpty(t *testing.T) {
	home := sandbox(t)
	srcDir := defaultSource(home)

	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)
	addSkillItem(t, srcDir, "fresh-skill")

	out, err := runCLI(t, "version", "list", "fresh-skill")
	if err != nil {
		t.Fatalf("version list (empty): %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "no saved versions") {
		t.Errorf("expected 'no saved versions': %s", out)
	}
}

func TestVersion_UnknownItem(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	_, err := runCLI(t, "version", "list", "ghost-item")
	if err == nil {
		t.Error("expected error for unknown item in version list")
	}
}

// --- backup ---

func TestBackup_CreateAndList(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	// Create a real file in the target dir so backup.Create copies it.
	tgtDir := filepath.Join(home, ".claude")
	mkfile(t, filepath.Join(tgtDir, "marker.txt"), "backup marker")

	out, err := runCLI(t, "backup", "create")
	if err != nil {
		t.Fatalf("backup create: %v\noutput: %s", err, out)
	}

	// Output should be the snapshot directory path.
	snapshotDir := strings.TrimSpace(out)
	if fi, err := os.Stat(snapshotDir); err != nil || !fi.IsDir() {
		t.Errorf("snapshot dir not created at %q: %v", snapshotDir, err)
	}

	out, err = runCLI(t, "backup", "list")
	if err != nil {
		t.Fatalf("backup list: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "TIMESTAMP") {
		t.Errorf("expected 'TIMESTAMP' in backup list output: %s", out)
	}
	if !strings.Contains(out, "claude") {
		t.Errorf("expected target name 'claude' in backup list output: %s", out)
	}
}

func TestBackup_List_NoBackups(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	out, err := runCLI(t, "backup", "list")
	if err != nil {
		t.Fatalf("backup list (empty): %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "no backups") {
		t.Errorf("expected 'no backups' in output: %s", out)
	}
}

func TestBackup_Prune(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	tgtDir := filepath.Join(home, ".claude")
	mkfile(t, filepath.Join(tgtDir, "marker.txt"), "backup marker")

	// Create a backup, then prune it away.
	if _, err := runCLI(t, "backup", "create"); err != nil {
		t.Fatalf("backup create: %v", err)
	}

	out, err := runCLI(t, "backup", "prune", "--keep", "0")
	if err != nil {
		t.Fatalf("backup prune: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "pruned") {
		t.Errorf("expected 'pruned' in output: %s", out)
	}

	// No backups should remain.
	out, err = runCLI(t, "backup", "list")
	if err != nil {
		t.Fatalf("backup list after prune: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "no backups") {
		t.Errorf("expected 'no backups' after prune: %s", out)
	}
}

func TestBackup_Restore(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	tgtDir := filepath.Join(home, ".claude")
	marker := filepath.Join(tgtDir, "restore-marker.txt")
	mkfile(t, marker, "restore test content")

	// Create backup snapshot.
	if _, err := runCLI(t, "backup", "create"); err != nil {
		t.Fatalf("backup create: %v", err)
	}

	// Remove the marker to simulate data loss.
	if err := os.Remove(marker); err != nil {
		t.Fatalf("remove marker: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("marker should be gone before restore")
	}

	// Restore from the latest snapshot.
	out, err := runCLI(t, "backup", "restore", "--latest")
	if err != nil {
		t.Fatalf("backup restore --latest: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "restored") {
		t.Errorf("expected 'restored' in output: %s", out)
	}

	// Marker should be back.
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("marker not restored: %v", err)
	}
	if string(data) != "restore test content" {
		t.Errorf("restored content mismatch: %q", data)
	}
}

// --- edit ---

func TestEdit_NoopEditor(t *testing.T) {
	home := sandbox(t)
	srcDir := defaultSource(home)

	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)
	addContextItem(t, srcDir, "CLAUDE.md", "# edit test")

	// Use the `true` command as a no-op editor so no interactive session opens.
	t.Setenv("EDITOR", "true")
	t.Setenv("VISUAL", "")

	_, err := runCLI(t, "edit", "CLAUDE.md")
	if err != nil {
		t.Fatalf("edit with noop editor: %v", err)
	}
}

func TestEdit_UnknownItem(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	_, err := runCLI(t, "edit", "ghost-item")
	if err == nil {
		t.Error("expected error editing unknown item")
	}
}
