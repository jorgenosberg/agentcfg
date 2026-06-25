package paths

import (
	"os"
	"testing"
)

func TestHome_default(t *testing.T) {
	// Ensure AGENTCFG_HOME is not set so we exercise the fallback path.
	orig, wasSet := os.LookupEnv("AGENTCFG_HOME")
	if wasSet {
		os.Unsetenv("AGENTCFG_HOME")
		t.Cleanup(func() { os.Setenv("AGENTCFG_HOME", orig) })
	}

	got, err := Home()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("unexpected error from UserHomeDir: %v", err)
	}
	if got != want {
		t.Errorf("Home() = %q, want %q", got, want)
	}
}

func TestHome_override(t *testing.T) {
	t.Setenv("AGENTCFG_HOME", "/tmp/my-sandbox")
	got, err := Home()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/tmp/my-sandbox" {
		t.Errorf("Home() = %q, want /tmp/my-sandbox", got)
	}
}
