package catalog_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jorgenosberg/agentcfg/internal/catalog"
	"github.com/jorgenosberg/agentcfg/internal/source"
)

func TestKnownAgentsHaveAgentField(t *testing.T) {
	for _, a := range catalog.KnownAgents() {
		if a.Agent == "" {
			t.Errorf("KnownAgents: target %q has empty Agent field", a.Name)
		}
	}
}

func TestKnownAgentsHaveAliasMatchingAgent(t *testing.T) {
	for _, a := range catalog.KnownAgents() {
		if a.Alias != a.Agent {
			t.Errorf("KnownAgents: target %q has Alias=%q, want %q", a.Name, a.Alias, a.Agent)
		}
	}
}

func TestKnownAgentResourceSubdirs(t *testing.T) {
	want := map[string]map[string]string{
		"copilot": {
			source.KindSkill: "skills",
		},
		"opencode": {
			source.KindSkill:     "skills",
			source.KindExtension: "plugins",
		},
		"pi": {
			source.KindSkill:     "skills",
			source.KindCommand:   "prompts",
			source.KindExtension: "extensions",
		},
	}

	for _, target := range catalog.KnownAgents() {
		expected, ok := want[target.Name]
		if !ok {
			continue
		}
		for kind, subdir := range expected {
			if got := target.Subdirs[kind]; got != subdir {
				t.Errorf("%s %s subdir: want %q got %q", target.Name, kind, subdir, got)
			}
		}
		delete(want, target.Name)
	}
	for name := range want {
		t.Errorf("missing catalog target %q", name)
	}
}

func TestTargetForClaude(t *testing.T) {
	tgt := catalog.TargetFor("claude", "/home/user/.claude-work", "claude-work")
	if tgt.Name != "claude-work" {
		t.Errorf("Name: want %q got %q", "claude-work", tgt.Name)
	}
	if tgt.Path != "/home/user/.claude-work" {
		t.Errorf("Path: want %q got %q", "/home/user/.claude-work", tgt.Path)
	}
	if tgt.Agent != "claude" {
		t.Errorf("Agent: want %q got %q", "claude", tgt.Agent)
	}
	if tgt.Alias != "claude" {
		t.Errorf("Alias: want %q got %q", "claude", tgt.Alias)
	}
	if tgt.Subdirs != nil {
		t.Error("Subdirs should be nil — profile drives defaults")
	}
}

func TestTargetForUnknownAgent(t *testing.T) {
	tgt := catalog.TargetFor("unknown-agent", "/tmp/foo", "foo")
	if tgt.Agent != "unknown-agent" {
		t.Errorf("Agent: want %q got %q", "unknown-agent", tgt.Agent)
	}
	if tgt.Alias != "unknown-agent" {
		t.Errorf("Alias: want %q got %q", "unknown-agent", tgt.Alias)
	}
}

func TestTargetForEmpty(t *testing.T) {
	tgt := catalog.TargetFor("", "/tmp/bar", "bar")
	if tgt.Agent != "" {
		t.Errorf("Agent: want empty, got %q", tgt.Agent)
	}
	if tgt.Alias != "" {
		t.Errorf("Alias: want empty when agent is empty, got %q", tgt.Alias)
	}
}

func TestKnownAgents_PathsUnderSandbox(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENTCFG_HOME", dir)

	for _, a := range catalog.KnownAgents() {
		if !strings.HasPrefix(a.Path, dir) {
			t.Errorf("KnownAgents: %q path %q not under sandbox %q", a.Name, a.Path, dir)
		}
	}
}

func TestDiscover_WithSandbox(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENTCFG_HOME", dir)

	// Create three of the catalog dirs; the others should not be discovered.
	for _, sub := range []string{".claude", ".codex", ".copilot"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	found := catalog.Discover()
	if len(found) != 3 {
		t.Fatalf("expected 3 discovered agents, got %d: %v", len(found), found)
	}

	names := map[string]bool{}
	for _, a := range found {
		names[a.Name] = true
	}
	for _, want := range []string{"claude", "codex", "copilot"} {
		if !names[want] {
			t.Errorf("Discover: expected %q in results, got %v", want, names)
		}
	}
}
