package source_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jorgenosberg/agentcfg/internal/source"
)

// mkfile writes a file at path (creating parent dirs).
func mkfile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdirall: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// mkdir creates a directory.
func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func TestScanNonexistentRoot(t *testing.T) {
	items, err := source.Scan("/nonexistent/path/that/does/not/exist")
	if err != nil {
		t.Fatalf("expected nil error for missing root, got %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty result, got %d items", len(items))
	}
}

func TestScanEmptyRoot(t *testing.T) {
	root := t.TempDir()
	items, err := source.Scan(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty result, got %d items", len(items))
	}
}

func TestScanSkills(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "skills", "deploy"))
	mkdir(t, filepath.Join(root, "skills", "review"))
	// a file in skills/ should be ignored (skills must be dirs)
	mkfile(t, filepath.Join(root, "skills", "not-a-skill.md"), "")

	items, err := source.Scan(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var skills []source.Item
	for _, it := range items {
		if it.Kind == source.KindSkill {
			skills = append(skills, it)
		}
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
	if skills[0].Name != "deploy" || skills[1].Name != "review" {
		t.Errorf("unexpected skill names: %v", []string{skills[0].Name, skills[1].Name})
	}
}

func TestScanHooks(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "hooks", "post-commit"), "#!/bin/sh")
	mkfile(t, filepath.Join(root, "hooks", "pre-push"), "#!/bin/sh")
	// a directory inside hooks/ should be ignored (hooks must be files)
	mkdir(t, filepath.Join(root, "hooks", "notahook"))

	items, err := source.Scan(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var hooks []source.Item
	for _, it := range items {
		if it.Kind == source.KindHook {
			hooks = append(hooks, it)
		}
	}
	if len(hooks) != 2 {
		t.Fatalf("expected 2 hooks, got %d", len(hooks))
	}
}

func TestScanContext(t *testing.T) {
	root := t.TempDir()
	// context/ with a non-empty subdir: all non-hidden files included
	mkfile(t, filepath.Join(root, "context", "CLAUDE.md"), "# hi")
	mkfile(t, filepath.Join(root, "context", "notes.txt"), "txt")
	mkfile(t, filepath.Join(root, "context", ".hidden"), "skip")

	items, err := source.Scan(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var ctx []source.Item
	for _, it := range items {
		if it.Kind == source.KindContext {
			ctx = append(ctx, it)
		}
	}
	if len(ctx) != 2 {
		t.Fatalf("expected 2 context items, got %d: %v", len(ctx), ctx)
	}
}

func TestScanContextRootMarkdownOnly(t *testing.T) {
	root := t.TempDir()
	// context subdir = "" means scan root; only .md files included
	sd := source.Subdirs{
		source.KindContext: "",
	}
	mkfile(t, filepath.Join(root, "CLAUDE.md"), "# hi")
	mkfile(t, filepath.Join(root, "GEMINI.md"), "# gemini")
	mkfile(t, filepath.Join(root, "notes.txt"), "not md")

	items, err := source.ScanWith(root, sd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 .md files, got %d", len(items))
	}
	for _, it := range items {
		if it.Kind != source.KindContext {
			t.Errorf("unexpected kind %s", it.Kind)
		}
	}
}

func TestScanHiddenFilesSkipped(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "skills", ".hidden-skill"))
	mkfile(t, filepath.Join(root, "hooks", ".hidden-hook"), "")
	mkfile(t, filepath.Join(root, "context", ".hidden-ctx"), "")

	items, err := source.Scan(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected no items (all hidden), got %d: %v", len(items), items)
	}
}

func TestScanSortedByKindThenName(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "skills", "zzz"))
	mkdir(t, filepath.Join(root, "skills", "aaa"))
	mkfile(t, filepath.Join(root, "hooks", "z-hook"), "")
	mkfile(t, filepath.Join(root, "hooks", "a-hook"), "")

	items, err := source.Scan(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// hooks < skills alphabetically
	for i := 1; i < len(items); i++ {
		prev, cur := items[i-1], items[i]
		if prev.Kind > cur.Kind || (prev.Kind == cur.Kind && prev.Name > cur.Name) {
			t.Errorf("items out of order at %d: %s/%s > %s/%s", i, prev.Kind, prev.Name, cur.Kind, cur.Name)
		}
	}
}

func TestScanWithEmptySubdirMap(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "something.md"), "")

	items, err := source.ScanWith(root, source.Subdirs{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("empty subdir map should find nothing, got %d items", len(items))
	}
}

func TestScanProjectNonexistentRoot(t *testing.T) {
	items, err := source.ScanProject("/no/such/path", "myproject")
	if err != nil {
		t.Fatalf("expected nil error for missing root, got %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty result, got %d items", len(items))
	}
}

func TestScanProjectClaudeFiles(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "CLAUDE.md"), "# context")
	mkdir(t, filepath.Join(root, ".claude", "skills", "deploy"))
	mkfile(t, filepath.Join(root, ".claude", "hooks", "post-tool-use"), "#!/bin/sh")

	items, err := source.ScanProject(root, "myproject")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byPath := map[string]source.ProjectItem{}
	for _, it := range items {
		byPath[it.RelPath] = it
	}

	if _, ok := byPath["CLAUDE.md"]; !ok {
		t.Error("expected CLAUDE.md in project items")
	}
	if _, ok := byPath[filepath.Join(".claude", "skills", "deploy")]; !ok {
		t.Error("expected .claude/skills/deploy in project items")
	}
	if _, ok := byPath[filepath.Join(".claude", "hooks", "post-tool-use")]; !ok {
		t.Error("expected .claude/hooks/post-tool-use in project items")
	}

	for _, it := range items {
		if it.Project != "myproject" {
			t.Errorf("expected project=myproject, got %q", it.Project)
		}
	}
}

func TestScanProjectHiddenEntriesSkipped(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, ".claude", "skills", ".hidden"))

	items, err := source.ScanProject(root, "proj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, it := range items {
		if it.Name == ".hidden" {
			t.Error("hidden entry should be skipped by ScanProject")
		}
	}
}

func TestScanProjectMultipleAgents(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "CLAUDE.md"), "claude")
	mkfile(t, filepath.Join(root, "AGENTS.md"), "agents")
	mkfile(t, filepath.Join(root, "GEMINI.md"), "gemini")
	mkfile(t, filepath.Join(root, ".cursorrules"), "cursor")

	items, err := source.ScanProject(root, "multi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agents := map[string]bool{}
	for _, it := range items {
		agents[it.Agent] = true
	}
	for _, want := range []string{"claude", "agents", "gemini", "cursor"} {
		if !agents[want] {
			t.Errorf("expected agent %q in project items", want)
		}
	}
}
