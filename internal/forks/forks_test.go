package forks_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jorgenosberg/agentcfg/internal/forks"
)

func TestLoad_EmptyWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "forks.json")
	f, err := forks.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f == nil || len(f.Forks) != 0 {
		t.Error("expected empty ForkFile for missing file")
	}
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "forks.json")
	now := time.Now().UTC().Truncate(time.Second)

	original := &forks.ForkFile{
		Version: 1,
		Forks: map[string]forks.Fork{
			"alpha@official": {
				ForkedAt:       now,
				SourceVersion:  "abc123",
				PluginDisabled: true,
				Skills:         []string{"skill-a", "skill-b"},
				Hooks:          []string{"hook-x"},
				Skipped: forks.Skipped{
					MCPServers: []string{"mcp-1"},
				},
			},
		},
	}

	if err := forks.Save(path, original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := forks.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	fork, ok := loaded.Forks["alpha@official"]
	if !ok {
		t.Fatal("fork not found after round-trip")
	}
	if !fork.ForkedAt.Equal(now) {
		t.Errorf("ForkedAt: got %v want %v", fork.ForkedAt, now)
	}
	if fork.SourceVersion != "abc123" {
		t.Errorf("SourceVersion: got %q", fork.SourceVersion)
	}
	if len(fork.Skills) != 2 || fork.Skills[0] != "skill-a" {
		t.Errorf("Skills: %v", fork.Skills)
	}
	if len(fork.Skipped.MCPServers) != 1 || fork.Skipped.MCPServers[0] != "mcp-1" {
		t.Errorf("Skipped.MCPServers: %v", fork.Skipped.MCPServers)
	}
}

func TestSave_CreatesParentDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subdir", "forks.json")
	f := &forks.ForkFile{Version: 1, Forks: map[string]forks.Fork{}}
	if err := forks.Save(path, f); err != nil {
		t.Fatalf("Save with missing parent: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

func TestIsForked(t *testing.T) {
	f := &forks.ForkFile{
		Version: 1,
		Forks: map[string]forks.Fork{
			"alpha@official": {Skills: []string{"skill-a", "skill-b"}},
		},
	}

	if !f.IsForked("alpha@official", "skill-a") {
		t.Error("skill-a should be forked")
	}
	if f.IsForked("alpha@official", "skill-c") {
		t.Error("skill-c should not be forked")
	}
	if f.IsForked("beta@official", "skill-a") {
		t.Error("unknown plugin should not be forked")
	}
}

func TestPluginForked(t *testing.T) {
	f := &forks.ForkFile{
		Version: 1,
		Forks: map[string]forks.Fork{
			"alpha@official": {},
		},
	}
	if !f.PluginForked("alpha@official") {
		t.Error("alpha should be marked forked")
	}
	if f.PluginForked("beta@official") {
		t.Error("beta should not be marked forked")
	}
}
