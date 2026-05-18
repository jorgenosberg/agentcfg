package catalog_test

import (
	"testing"

	"github.com/jorgenosberg/agentcfg/internal/catalog"
)

func TestKnownAgentsHaveAgentField(t *testing.T) {
	for _, a := range catalog.KnownAgents() {
		if a.Agent == "" {
			t.Errorf("KnownAgents: target %q has empty Agent field", a.Name)
		}
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
	if tgt.Subdirs != nil {
		t.Error("Subdirs should be nil — profile drives defaults")
	}
}

func TestTargetForUnknownAgent(t *testing.T) {
	tgt := catalog.TargetFor("unknown-agent", "/tmp/foo", "foo")
	if tgt.Agent != "unknown-agent" {
		t.Errorf("Agent: want %q got %q", "unknown-agent", tgt.Agent)
	}
}

func TestTargetForEmpty(t *testing.T) {
	tgt := catalog.TargetFor("", "/tmp/bar", "bar")
	if tgt.Agent != "" {
		t.Errorf("Agent: want empty, got %q", tgt.Agent)
	}
}
