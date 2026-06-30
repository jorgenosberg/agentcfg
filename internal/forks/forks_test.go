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
	if f.Version != 2 {
		t.Errorf("Version: got %d want 2", f.Version)
	}
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "forks.json")
	now := time.Now().UTC().Truncate(time.Second)

	original := &forks.ForkFile{
		Version: 2,
		Forks: map[string]forks.Fork{
			"alpha@official": {
				ForkedAt:         now,
				SourceVersion:    "abc123",
				UpstreamFullName: "alpha@official",
				ForkFullName:     "alpha@agentcfg-forks",
				BundlePath:       "/tmp/forks/plugins/alpha",
				UpstreamDisabled: true,
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
	if fork.ForkFullName != "alpha@agentcfg-forks" {
		t.Errorf("ForkFullName: got %q", fork.ForkFullName)
	}
	if fork.BundlePath != "/tmp/forks/plugins/alpha" {
		t.Errorf("BundlePath: got %q", fork.BundlePath)
	}
	if !fork.UpstreamDisabled {
		t.Error("UpstreamDisabled should be true")
	}
}

func TestSave_CreatesParentDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subdir", "forks.json")
	f := &forks.ForkFile{Version: 2, Forks: map[string]forks.Fork{}}
	if err := forks.Save(path, f); err != nil {
		t.Fatalf("Save with missing parent: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

func TestPluginForked(t *testing.T) {
	f := &forks.ForkFile{
		Version: 2,
		Forks: map[string]forks.Fork{
			"alpha@official": {ForkFullName: "alpha@agentcfg-forks"},
		},
	}
	if !f.PluginForked("alpha@official") {
		t.Error("alpha should be marked forked")
	}
	if f.PluginForked("beta@official") {
		t.Error("beta should not be marked forked")
	}

	var nilFF *forks.ForkFile
	if nilFF.PluginForked("anything") {
		t.Error("nil ForkFile should return false")
	}
}
