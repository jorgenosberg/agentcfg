package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jorgenosberg/agentcfg/internal/config"
)

// targetSubdirConfig builds a config whose target uses the claude agent profile.
// The import command now calls t.SupportedSubdirs(), so no explicit Subdirs override
// is needed — the agent profile provides the full layout including commands.
func targetSubdirConfig(home, tgtPath string) config.Config {
	return config.Config{
		Source:          defaultSource(home),
		DefaultStrategy: config.StrategyLink,
		Projects:        []config.Project{},
		Targets: []config.Target{
			{
				Name:  "claude",
				Path:  tgtPath,
				Agent: "claude",
				Alias: "claude",
			},
		},
	}
}

func TestImport_SingleContextItem(t *testing.T) {
	home := sandbox(t)
	tgtPath := filepath.Join(home, ".claude")

	cfg := targetSubdirConfig(home, tgtPath)
	seedConfig(t, home, cfg)
	seedTargetTree(t, tgtPath)

	out, err := runCLI(t, "import", "claude", "CLAUDE.md")
	if err != nil {
		t.Fatalf("import: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "imported") {
		t.Errorf("expected 'imported' in output: %s", out)
	}

	dest := filepath.Join(defaultSource(home), "context", "CLAUDE.md")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("imported context item not at %s: %v", dest, err)
	}
}

func TestImport_SingleHookItem(t *testing.T) {
	home := sandbox(t)
	tgtPath := filepath.Join(home, ".claude")

	cfg := targetSubdirConfig(home, tgtPath)
	seedConfig(t, home, cfg)
	seedTargetTree(t, tgtPath)

	out, err := runCLI(t, "import", "claude", "pre-tool-call.sh")
	if err != nil {
		t.Fatalf("import hook: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "imported") {
		t.Errorf("expected 'imported' in output: %s", out)
	}

	dest := filepath.Join(defaultSource(home), "hooks", "pre-tool-call.sh")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("imported hook not at %s: %v", dest, err)
	}
}

func TestImport_SkillDirectoryCopy(t *testing.T) {
	home := sandbox(t)
	tgtPath := filepath.Join(home, ".claude")

	cfg := targetSubdirConfig(home, tgtPath)
	seedConfig(t, home, cfg)
	seedTargetTree(t, tgtPath)

	out, err := runCLI(t, "import", "claude", "my-skill")
	if err != nil {
		t.Fatalf("import skill dir: %v\noutput: %s", err, out)
	}

	// Skill items are directories; verify the copy is a real directory.
	dest := filepath.Join(defaultSource(home), "skills", "my-skill")
	fi, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("imported skill dir not at %s: %v", dest, err)
	}
	if !fi.IsDir() {
		t.Error("imported skill item should be a directory")
	}

	// Content inside the copied dir should be present.
	inner := filepath.Join(dest, "SKILL.md")
	if _, err := os.Stat(inner); err != nil {
		t.Errorf("SKILL.md not inside imported skill dir: %v", err)
	}
}

func TestImport_MultipleItems(t *testing.T) {
	home := sandbox(t)
	tgtPath := filepath.Join(home, ".claude")

	cfg := targetSubdirConfig(home, tgtPath)
	seedConfig(t, home, cfg)
	seedTargetTree(t, tgtPath)

	out, err := runCLI(t, "import", "claude", "CLAUDE.md", "pre-tool-call.sh")
	if err != nil {
		t.Fatalf("import multiple: %v\noutput: %s", err, out)
	}

	if strings.Count(out, "imported") != 2 {
		t.Errorf("expected 2 'imported' lines, got: %s", out)
	}

	for _, p := range []string{
		filepath.Join(defaultSource(home), "context", "CLAUDE.md"),
		filepath.Join(defaultSource(home), "hooks", "pre-tool-call.sh"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected imported item at %s: %v", p, err)
		}
	}
}

func TestImport_All(t *testing.T) {
	home := sandbox(t)
	tgtPath := filepath.Join(home, ".claude")

	cfg := targetSubdirConfig(home, tgtPath)
	seedConfig(t, home, cfg)
	seedTargetTree(t, tgtPath)

	out, err := runCLI(t, "import", "claude", "--all")
	if err != nil {
		t.Fatalf("import --all: %v\noutput: %s", err, out)
	}

	for _, p := range []string{
		filepath.Join(defaultSource(home), "skills", "my-skill"),
		filepath.Join(defaultSource(home), "hooks", "pre-tool-call.sh"),
		filepath.Join(defaultSource(home), "context", "CLAUDE.md"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected imported item at %s: %v", p, err)
		}
	}
}

func TestImport_SkipExistsWithoutForce(t *testing.T) {
	home := sandbox(t)
	tgtPath := filepath.Join(home, ".claude")

	cfg := targetSubdirConfig(home, tgtPath)
	seedConfig(t, home, cfg)
	seedTargetTree(t, tgtPath)

	// Pre-create the destination with original content.
	dest := filepath.Join(defaultSource(home), "context", "CLAUDE.md")
	mkfile(t, dest, "original content")

	out, err := runCLI(t, "import", "claude", "CLAUDE.md")
	if err != nil {
		t.Fatalf("import (skip): %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "skip") {
		t.Errorf("expected 'skip' in output: %s", out)
	}

	// Original content must be unchanged.
	data, _ := os.ReadFile(dest)
	if string(data) != "original content" {
		t.Errorf("skip should preserve existing content, got %q", data)
	}
}

func TestImport_ForceOverwrites(t *testing.T) {
	home := sandbox(t)
	tgtPath := filepath.Join(home, ".claude")

	cfg := targetSubdirConfig(home, tgtPath)
	seedConfig(t, home, cfg)
	seedTargetTree(t, tgtPath)

	// Pre-create the destination with stale content.
	dest := filepath.Join(defaultSource(home), "context", "CLAUDE.md")
	mkfile(t, dest, "stale content")

	out, err := runCLI(t, "import", "claude", "--force", "CLAUDE.md")
	if err != nil {
		t.Fatalf("import --force: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "imported") {
		t.Errorf("expected 'imported' in output: %s", out)
	}

	// Content should be from the target tree, not the stale pre-existing file.
	data, _ := os.ReadFile(dest)
	if string(data) == "stale content" {
		t.Error("--force should overwrite stale content")
	}
}

func TestImport_SingleCommandItem(t *testing.T) {
	home := sandbox(t)
	tgtPath := filepath.Join(home, ".claude")

	cfg := targetSubdirConfig(home, tgtPath)
	seedConfig(t, home, cfg)
	seedTargetTree(t, tgtPath)

	out, err := runCLI(t, "import", "claude", "review.md")
	if err != nil {
		t.Fatalf("import command: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "imported") {
		t.Errorf("expected 'imported' in output: %s", out)
	}

	dest := filepath.Join(defaultSource(home), "commands", "review.md")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("imported command not at %s: %v", dest, err)
	}
}

func TestImport_NoItemsNoAll(t *testing.T) {
	home := sandbox(t)
	tgtPath := filepath.Join(home, ".claude")

	cfg := targetSubdirConfig(home, tgtPath)
	seedConfig(t, home, cfg)

	_, err := runCLI(t, "import", "claude")
	if err == nil {
		t.Error("expected error when no items and no --all")
	}
}

func TestImport_UnknownTarget(t *testing.T) {
	home := sandbox(t)
	cfg := defaultConfig(home)
	seedConfig(t, home, cfg)

	_, err := runCLI(t, "import", "nonexistent", "--all")
	if err == nil {
		t.Error("expected error for unknown target")
	}
}

func TestImport_ItemNotFound(t *testing.T) {
	home := sandbox(t)
	tgtPath := filepath.Join(home, ".claude")

	cfg := targetSubdirConfig(home, tgtPath)
	seedConfig(t, home, cfg)
	seedTargetTree(t, tgtPath)

	_, err := runCLI(t, "import", "claude", "nonexistent-item")
	if err == nil {
		t.Error("expected error for item not found in target")
	}
}
