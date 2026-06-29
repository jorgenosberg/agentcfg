package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/jorgenosberg/agentcfg/internal/cli"
	"github.com/jorgenosberg/agentcfg/internal/config"
)

// sandbox sets AGENTCFG_HOME to a fresh TempDir, isolating all path
// resolution (config, catalog, plugin files) from the real home.
// Returns the sandbox dir.
func sandbox(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AGENTCFG_HOME", dir)
	return dir
}

// runCLI executes the agentcfg root command with the given args,
// returning captured stdout+stderr and any error.
func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	root := cli.NewRoot()
	root.SetArgs(args)
	root.SetOut(&buf)
	root.SetErr(&buf)
	err := root.Execute()
	return buf.String(), err
}

// seedConfig saves cfg to ~/.agentcfg/config.json under home
// and creates the standard source subdirectories.
func seedConfig(t *testing.T, home string, cfg config.Config) {
	t.Helper()
	cfgPath := filepath.Join(home, ".agentcfg", "config.json")
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("seedConfig: %v", err)
	}
	for _, sub := range []string{"skills", "hooks", "context"} {
		if err := os.MkdirAll(filepath.Join(cfg.Source, sub), 0o755); err != nil {
			t.Fatalf("seedConfig mkdir %s: %v", sub, err)
		}
	}
}

// mkfile writes content to path, creating parent dirs as needed.
func mkfile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdirall %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("mkfile %s: %v", path, err)
	}
}

// addContextItem creates a context file under $srcDir/context/.
// Context files are installed to the root of each target dir.
func addContextItem(t *testing.T, srcDir, name, content string) {
	t.Helper()
	mkfile(t, filepath.Join(srcDir, "context", name), content)
}

// defaultSource returns the standard source path under the sandbox.
func defaultSource(home string) string {
	return filepath.Join(home, ".agentcfg", "source")
}

// defaultConfig returns a minimal config with one claude target pointing
// at $home/.claude. Agent profile is "claude" (link strategy, profile-derived subdirs).
func defaultConfig(home string) config.Config {
	return config.Config{
		Source:          defaultSource(home),
		DefaultStrategy: config.StrategyLink,
		Projects:        []config.Project{},
		Targets: []config.Target{
			{
				Name:  "claude",
				Path:  filepath.Join(home, ".claude"),
				Agent: "claude",
				Alias: "claude",
			},
		},
	}
}
