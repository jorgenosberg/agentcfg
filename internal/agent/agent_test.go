package agent_test

import (
	"testing"

	"github.com/jorgenosberg/agentcfg/internal/agent"
)

func TestGetKnownAgent(t *testing.T) {
	p, ok := agent.Get("claude")
	if !ok {
		t.Fatal("expected claude to be in registry")
	}
	if p.Subdirs["skill"] != "skills" {
		t.Errorf("claude skill subdir: want %q got %q", "skills", p.Subdirs["skill"])
	}
	if p.Subdirs["hook"] != "hooks" {
		t.Errorf("claude hook subdir: want %q got %q", "hooks", p.Subdirs["hook"])
	}
}

func TestGetUnknownAgent(t *testing.T) {
	_, ok := agent.Get("doesnotexist")
	if ok {
		t.Error("expected false for unknown agent name")
	}
}

func TestNamesSorted(t *testing.T) {
	names := agent.Names()
	if len(names) == 0 {
		t.Fatal("expected non-empty names list")
	}
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("names not sorted at index %d: %q < %q", i, names[i], names[i-1])
		}
	}
}

func TestNamesContainsAll(t *testing.T) {
	expected := []string{"aider", "agents", "cline", "claude", "codex", "copilot", "cursor", "gemini", "opencode", "windsurf"}
	names := agent.Names()
	got := make(map[string]bool, len(names))
	for _, n := range names {
		got[n] = true
	}
	for _, e := range expected {
		if !got[e] {
			t.Errorf("missing agent name: %q", e)
		}
	}
}
