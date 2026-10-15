package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jorgenosberg/agentcfg/internal/config"
)

func fileHas(t *testing.T, path, sub string) bool {
	t.Helper()
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Contains(string(raw), sub)
}

func TestHookRegistration_SyncAndMoveRoundTrip(t *testing.T) {
	home := sandbox(t)
	src := filepath.Join(home, ".agentcfg", "source")
	mkfile(t, filepath.Join(src, "hooks", "guard.sh"), "#!/bin/sh\n# agentcfg-hook: PreToolUse Bash timeout=10\n")
	mkfile(t, filepath.Join(src, "hooks", "plain.sh"), "#!/bin/sh\n")
	repo := filepath.Join(home, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		Source:          src,
		DefaultStrategy: config.StrategyLink,
		Projects:        []config.Project{{Name: "repo", Path: repo}},
		Targets: []config.Target{
			{Name: "claude", Agent: "claude", Path: filepath.Join(home, ".claude")},
			{Name: "codex", Agent: "codex", Path: filepath.Join(home, ".codex")},
			{Name: "copilot", Agent: "copilot", Path: filepath.Join(home, ".copilot")},
		},
	}
	if err := config.Save(filepath.Join(home, ".agentcfg", "config.json"), cfg); err != nil {
		t.Fatal(err)
	}
	global := map[string]string{
		filepath.Join(home, ".claude", "settings.json"):           filepath.Join(home, ".claude", "hooks", "guard.sh"),
		filepath.Join(home, ".codex", "hooks.json"):               filepath.Join(home, ".codex", "hooks", "guard.sh"),
		filepath.Join(home, ".copilot", "hooks", "agentcfg.json"): filepath.Join(home, ".copilot", "hooks", "guard.sh"),
	}

	out, err := runCLI(t, "sync", "--no-backup")
	if err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	for file, cmd := range global {
		if !fileHas(t, file, cmd) {
			t.Errorf("%s does not register %s", file, cmd)
		}
		if fileHas(t, file, "plain.sh") {
			t.Errorf("%s registers a script without a header", file)
		}
	}
	if !strings.Contains(out, "/hooks") {
		t.Errorf("missing Codex trust note:\n%s", out)
	}

	out, err = runCLI(t, "status")
	if err != nil || strings.Contains(out, "HOOK REGISTRATION") {
		t.Fatalf("status after sync should show no registration drift: %v\n%s", err, out)
	}

	out, err = runCLI(t, "move", "guard.sh", "--to", "repo", "--no-backup")
	if err != nil {
		t.Fatalf("move to project: %v\n%s", err, out)
	}
	for file := range global {
		if fileHas(t, file, "guard.sh") {
			t.Errorf("%s still registers guard.sh after moving it to the project", file)
		}
	}
	if !fileHas(t, filepath.Join(repo, ".claude", "settings.json"), `\"$CLAUDE_PROJECT_DIR\"/.claude/hooks/guard.sh`) {
		t.Error("project Claude settings missing guard.sh")
	}
	if !fileHas(t, filepath.Join(repo, ".codex", "hooks.json"), `.codex/hooks/guard.sh`) {
		t.Error("project Codex hooks missing guard.sh")
	}
	if _, err := os.Stat(filepath.Join(repo, ".github", "hooks")); !os.IsNotExist(err) {
		t.Error("Copilot project hook written although Copilot runs the Claude registration")
	}

	out, err = runCLI(t, "move", "guard.sh", "--to", "global", "--no-backup")
	if err != nil {
		t.Fatalf("move to global: %v\n%s", err, out)
	}
	for file, cmd := range global {
		if !fileHas(t, file, cmd) {
			t.Errorf("%s does not register %s after moving back", file, cmd)
		}
	}
	if fileHas(t, filepath.Join(repo, ".claude", "settings.json"), "guard.sh") || fileHas(t, filepath.Join(repo, ".codex", "hooks.json"), "guard.sh") {
		t.Error("project registrations left behind after moving to global")
	}
}
