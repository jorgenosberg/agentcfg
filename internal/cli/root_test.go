package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInit_NonInteractive(t *testing.T) {
	home := sandbox(t)

	out, err := runCLI(t, "init", "--no-interactive")
	if err != nil {
		t.Fatalf("init: %v\noutput: %s", err, out)
	}

	cfgPath := filepath.Join(home, ".agentcfg", "config.json")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Errorf("config.json not created at %s: %v", cfgPath, err)
	}

	for _, sub := range []string{"skills", "hooks", "context"} {
		d := filepath.Join(home, ".agentcfg", "source", sub)
		fi, err := os.Stat(d)
		if err != nil || !fi.IsDir() {
			t.Errorf("source subdir %q not created", sub)
		}
	}
}

func TestInit_AlreadyExists(t *testing.T) {
	sandbox(t)

	if _, err := runCLI(t, "init", "--no-interactive"); err != nil {
		t.Fatalf("first init: %v", err)
	}
	_, err := runCLI(t, "init", "--no-interactive")
	if err == nil {
		t.Error("expected error when config already exists, got nil")
	}
}

func TestDiscover_Paths(t *testing.T) {
	home := sandbox(t)

	// --paths returns early before loading config, so no init needed.
	out, err := runCLI(t, "discover", "--paths")
	if err != nil {
		t.Fatalf("discover --paths: %v\noutput: %s", err, out)
	}

	for line := range strings.SplitSeq(out, "\n") {
		if line == "" || strings.HasPrefix(strings.TrimSpace(line), "NAME") {
			continue
		}
		if !strings.Contains(line, home) {
			t.Errorf("catalog path not under sandbox %q: %q", home, line)
		}
	}
}

func TestDiscover_FindsAgentDirs(t *testing.T) {
	home := sandbox(t)

	// Create fake agent install dirs inside the sandbox.
	for _, d := range []string{".claude", ".codex"} {
		if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// init first so discover has a config to load.
	if _, err := runCLI(t, "init", "--no-interactive"); err != nil {
		t.Fatalf("init: %v", err)
	}

	out, err := runCLI(t, "discover")
	if err != nil {
		t.Fatalf("discover: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "claude") {
		t.Errorf("discover output missing 'claude': %s", out)
	}
	if !strings.Contains(out, "codex") {
		t.Errorf("discover output missing 'codex': %s", out)
	}
}

func TestInstallAndUninstall(t *testing.T) {
	home := sandbox(t)
	srcDir := defaultSource(home)

	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)
	addContextItem(t, srcDir, "CLAUDE.md", "# hello")

	out, err := runCLI(t, "install", "CLAUDE.md")
	if err != nil {
		t.Fatalf("install: %v\noutput: %s", err, out)
	}

	dest := filepath.Join(home, ".claude", "CLAUDE.md")
	fi, err := os.Lstat(dest)
	if err != nil {
		t.Fatalf("no file at dest after install: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink at dest (link strategy)")
	}

	out, err = runCLI(t, "uninstall", "CLAUDE.md")
	if err != nil {
		t.Fatalf("uninstall: %v\noutput: %s", err, out)
	}
	if _, err := os.Lstat(dest); !os.IsNotExist(err) {
		t.Error("expected dest removed after uninstall")
	}
}

func TestStatus_ShowsLinked(t *testing.T) {
	home := sandbox(t)
	srcDir := defaultSource(home)

	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)
	addContextItem(t, srcDir, "CLAUDE.md", "# hello")

	if _, err := runCLI(t, "install", "CLAUDE.md"); err != nil {
		t.Fatalf("install: %v", err)
	}

	out, err := runCLI(t, "status")
	if err != nil {
		t.Fatalf("status: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "linked") {
		t.Errorf("status output missing 'linked': %s", out)
	}
}

func TestSync_DryRun(t *testing.T) {
	home := sandbox(t)
	srcDir := defaultSource(home)

	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)
	addContextItem(t, srcDir, "CLAUDE.md", "# dry run test")

	out, err := runCLI(t, "sync", "--dry-run")
	if err != nil {
		t.Fatalf("sync --dry-run: %v\noutput: %s", err, out)
	}

	// dry-run must not write any files.
	dest := filepath.Join(home, ".claude", "CLAUDE.md")
	if _, err := os.Lstat(dest); !os.IsNotExist(err) {
		t.Error("dry-run should not install files")
	}
	if !strings.Contains(out, "absent") {
		t.Errorf("dry-run output should show 'absent': %s", out)
	}
}

func TestSync_InstallsItems(t *testing.T) {
	home := sandbox(t)
	srcDir := defaultSource(home)

	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)
	addContextItem(t, srcDir, "CLAUDE.md", "# sync test")

	out, err := runCLI(t, "sync", "--no-backup")
	if err != nil {
		t.Fatalf("sync: %v\noutput: %s", err, out)
	}

	dest := filepath.Join(home, ".claude", "CLAUDE.md")
	fi, err := os.Lstat(dest)
	if err != nil {
		t.Fatalf("item not installed by sync: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink at dest after sync (link strategy)")
	}

	// Lockfile must be written.
	lockPath := filepath.Join(home, ".agentcfg", "locks.json")
	if _, err := os.Stat(lockPath); err != nil {
		t.Errorf("locks.json not created: %v", err)
	}
}

func TestTarget_AddAndList(t *testing.T) {
	home := sandbox(t)

	if _, err := runCLI(t, "init", "--no-interactive"); err != nil {
		t.Fatalf("init: %v", err)
	}

	tgtDir := filepath.Join(home, "my-agent")
	if err := os.MkdirAll(tgtDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// target add <name> <path> --agent <type>
	out, err := runCLI(t, "target", "add", "my-agent", tgtDir, "--agent", "claude")
	if err != nil {
		t.Fatalf("target add: %v\noutput: %s", err, out)
	}

	out, err = runCLI(t, "target", "list")
	if err != nil {
		t.Fatalf("target list: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "my-agent") {
		t.Errorf("target list missing 'my-agent': %s", out)
	}
}
