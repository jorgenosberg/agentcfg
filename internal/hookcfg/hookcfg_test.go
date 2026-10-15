package hookcfg

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jorgenosberg/agentcfg/internal/config"
)

const superset = `[ -n "$SUPERSET_HOME_DIR" ] && "$SUPERSET_HOME_DIR/hooks/notify.sh" || true`

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func commands(doc map[string]any, event string) []string {
	var out []string
	hooks, _ := doc["hooks"].(map[string]any)
	groups, _ := hooks[event].([]any)
	for _, g := range groups {
		gm := g.(map[string]any)
		for _, h := range gm["hooks"].([]any) {
			out = append(out, gm["matcher"].(string)+" "+h.(map[string]any)["command"].(string))
		}
	}
	return out
}

func TestParse(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.sh")
	writeFile(t, p, "#!/usr/bin/env bash\n# x.sh\n#   agentcfg-hook: PreToolUse Bash timeout=10\n# agentcfg-hook: SessionStart\n\necho\n# agentcfg-hook: Stop\n")
	regs, err := Parse(p)
	if err != nil {
		t.Fatal(err)
	}
	want := []Registration{{Event: "PreToolUse", Matcher: "Bash", Timeout: 10}, {Event: "SessionStart"}}
	if len(regs) != len(want) || regs[0] != want[0] || regs[1] != want[1] {
		t.Fatalf("got %+v, want %+v (headers after the comment block are ignored)", regs, want)
	}

	writeFile(t, p, "#!/bin/sh\n# agentcfg-hook: PreToolUse Bash status=\"Checking repo git policy\" timeout=5\n")
	regs, err = Parse(p)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Registration{Event: "PreToolUse", Matcher: "Bash", Timeout: 5, Status: "Checking repo git policy"}); len(regs) != 1 || regs[0] != want {
		t.Fatalf("got %+v, want %+v", regs, want)
	}
	writeFile(t, p, "#!/bin/sh\n# agentcfg-hook: PreToolUse Bash status=\"open\n")
	if _, err := Parse(p); err == nil {
		t.Fatal("expected an error for an unclosed status")
	}

	writeFile(t, p, "#!/bin/sh\n# agentcfg-hook: PreToolUse Edit Write\n")
	if _, err := Parse(p); err == nil {
		t.Fatal("expected an error for a matcher with a space")
	}
	writeFile(t, p, "#!/bin/sh\n# agentcfg-hook: PreToolUse timeout=abc\n")
	if _, err := Parse(p); err == nil {
		t.Fatal("expected an error for a bad timeout")
	}
}

func TestReconcileClaudeKeepsForeignEntries(t *testing.T) {
	root := t.TempDir()
	tg := config.Target{Name: "claude", Agent: "claude", Path: filepath.Join(root, ".claude")}
	hooks := filepath.Join(tg.Path, "hooks")
	writeFile(t, filepath.Join(hooks, "guard.sh"), "#!/bin/sh\n# agentcfg-hook: PreToolUse Bash timeout=10\n")
	writeFile(t, filepath.Join(hooks, "manual.sh"), "#!/bin/sh\n")
	writeFile(t, filepath.Join(hooks, "kept.sh"), "#!/bin/sh\n# agentcfg-hook: PreToolUse Bash\n")
	settings := map[string]any{
		"model": "opus",
		"hooks": map[string]any{
			"PreToolUse": []any{map[string]any{"matcher": "Bash", "hooks": []any{
				map[string]any{"type": "command", "command": filepath.Join(hooks, "kept.sh")},
				map[string]any{"type": "command", "command": filepath.Join(hooks, "manual.sh")},
				map[string]any{"type": "command", "command": filepath.Join(hooks, "gone.sh")},
			}}},
			"Stop": []any{map[string]any{"hooks": []any{
				map[string]any{"type": "command", "command": filepath.Join(hooks, "gone.sh")},
			}}},
			"PostToolUse": []any{map[string]any{"matcher": "*", "hooks": []any{
				map[string]any{"type": "command", "command": superset},
			}}},
		},
	}
	raw, _ := json.Marshal(settings)
	file := filepath.Join(tg.Path, "settings.json")
	writeFile(t, file, string(raw))

	res := Reconcile(tg, false)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if !res.Written || len(res.Added) != 1 || len(res.Removed) != 2 {
		t.Fatalf("added %v removed %v written %v", res.Added, res.Removed, res.Written)
	}

	doc := readJSON(t, file)
	if doc["model"] != "opus" {
		t.Error("unrelated settings lost")
	}
	got := strings.Join(commands(doc, "PreToolUse"), "\n")
	want := strings.Join([]string{
		"Bash " + filepath.Join(hooks, "kept.sh"),
		"Bash " + filepath.Join(hooks, "manual.sh"),
		"Bash " + filepath.Join(hooks, "guard.sh"),
	}, "\n")
	if got != want {
		t.Errorf("PreToolUse:\n%s\nwant:\n%s", got, want)
	}
	if _, ok := doc["hooks"].(map[string]any)["Stop"]; ok {
		t.Error("empty Stop event should be removed")
	}
	if c := commands(doc, "PostToolUse"); len(c) != 1 || c[0] != "* "+superset {
		t.Errorf("foreign entry changed: %v", c)
	}
	body, _ := os.ReadFile(file)
	if strings.Contains(string(body), "\\u0026") {
		t.Errorf("shell operators were HTML-escaped:\n%s", body)
	}

	again := Reconcile(tg, false)
	if again.Err != nil || again.Written || again.Changed() {
		t.Fatalf("second run should be a no-op: %+v", again)
	}
}

func TestReconcileUpdatesTimeoutInPlace(t *testing.T) {
	root := t.TempDir()
	tg := config.Target{Name: "codex", Agent: "codex", Path: filepath.Join(root, ".codex")}
	script := filepath.Join(tg.Path, "hooks", "guard.sh")
	writeFile(t, script, "#!/bin/sh\n# agentcfg-hook: PreToolUse Bash timeout=30\n")
	file := filepath.Join(tg.Path, "hooks.json")
	writeFile(t, file, `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"`+script+`","statusMessage":"Checking","timeout":10}]}]}}`)

	res := Reconcile(tg, false)
	if res.Err != nil || len(res.Updated) != 1 || len(res.Added)+len(res.Removed) != 0 {
		t.Fatalf("%+v", res)
	}
	h := readJSON(t, file)["hooks"].(map[string]any)["PreToolUse"].([]any)[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)
	if h["timeout"] != float64(30) || h["statusMessage"] != "Checking" {
		t.Errorf("entry not updated in place: %v", h)
	}
}

func TestReconcileCodexStatusMessage(t *testing.T) {
	root := t.TempDir()
	tg := config.Target{Name: "codex", Agent: "codex", Path: filepath.Join(root, ".codex")}
	hooks := filepath.Join(tg.Path, "hooks")
	writeFile(t, filepath.Join(hooks, "a-plain.sh"), "#!/bin/sh\n# agentcfg-hook: PreToolUse Bash\n")
	writeFile(t, filepath.Join(hooks, "b-labeled.sh"), "#!/bin/sh\n# agentcfg-hook: PreToolUse Bash status=\"From header\"\n")
	writeFile(t, filepath.Join(hooks, "c-manual.sh"), "#!/bin/sh\n# agentcfg-hook: PreToolUse Bash\n")
	file := filepath.Join(tg.Path, "hooks.json")
	writeFile(t, file, `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[`+
		`{"type":"command","command":"`+filepath.Join(hooks, "b-labeled.sh")+`","statusMessage":"Old"},`+
		`{"type":"command","command":"`+filepath.Join(hooks, "c-manual.sh")+`","statusMessage":"Set by hand"}]}]}}`)

	if res := Reconcile(tg, false); res.Err != nil {
		t.Fatal(res.Err)
	}
	got := map[string]any{}
	for _, h := range readJSON(t, file)["hooks"].(map[string]any)["PreToolUse"].([]any)[0].(map[string]any)["hooks"].([]any) {
		hm := h.(map[string]any)
		got[filepath.Base(hm["command"].(string))] = hm["statusMessage"]
	}
	want := map[string]any{"a-plain.sh": "a-plain", "b-labeled.sh": "From header", "c-manual.sh": "Set by hand"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: statusMessage %v, want %v", k, got[k], v)
		}
	}

	// Claude has no label field, so none is written there.
	claude := config.Target{Name: "claude", Agent: "claude", Path: filepath.Join(root, ".claude")}
	writeFile(t, filepath.Join(claude.Path, "hooks", "a.sh"), "#!/bin/sh\n# agentcfg-hook: PreToolUse Bash status=\"X\"\n")
	if res := Reconcile(claude, false); res.Err != nil {
		t.Fatal(res.Err)
	}
	if fileBody, _ := os.ReadFile(filepath.Join(claude.Path, "settings.json")); strings.Contains(string(fileBody), "statusMessage") {
		t.Error("statusMessage written to Claude settings")
	}
}

func TestReconcileReportsForeignDuplicate(t *testing.T) {
	root := t.TempDir()
	tg := config.Target{Name: "codex", Agent: "codex", Path: filepath.Join(root, ".codex")}
	writeFile(t, filepath.Join(tg.Path, "hooks", "pr-title-check.sh"), "#!/bin/sh\n# agentcfg-hook: PreToolUse Bash\n")
	other := filepath.Join(root, ".claude", "hooks", "pr-title-check.sh")
	writeFile(t, filepath.Join(tg.Path, "hooks.json"), `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"`+other+`"}]}]}}`)

	res := Reconcile(tg, true)
	if res.Err != nil || len(res.Foreign) != 1 || res.Foreign[0].Command != other || len(res.Added) != 1 {
		t.Fatalf("%+v", res)
	}
	if res.Written {
		t.Error("dry run wrote the file")
	}
}

func TestReconcileProjectCommands(t *testing.T) {
	repo := t.TempDir()
	p := config.Project{Name: "repo", Path: repo}
	claude, _ := config.ProjectTarget(p, "claude", false)
	codex, _ := config.ProjectTarget(p, "codex", false)
	writeFile(t, filepath.Join(repo, ".claude", "hooks", "g.sh"), "#!/bin/sh\n# agentcfg-hook: PreToolUse Bash\n")
	writeFile(t, filepath.Join(repo, ".codex", "hooks", "g.sh"), "#!/bin/sh\n# agentcfg-hook: PreToolUse Bash\n")

	for _, r := range ReconcileAll([]config.Target{claude, codex}, false) {
		if r.Err != nil {
			t.Fatal(r.Err)
		}
	}
	if c := commands(readJSON(t, filepath.Join(repo, ".claude", "settings.json")), "PreToolUse"); len(c) != 1 || c[0] != `Bash "$CLAUDE_PROJECT_DIR"/.claude/hooks/g.sh` {
		t.Errorf("claude: %v", c)
	}
	if c := commands(readJSON(t, filepath.Join(repo, ".codex", "hooks.json")), "PreToolUse"); len(c) != 1 || c[0] != `Bash "$(git rev-parse --show-toplevel)"/.codex/hooks/g.sh` {
		t.Errorf("codex: %v", c)
	}

	// Removing the script removes the registration it owned.
	if err := os.Remove(filepath.Join(repo, ".claude", "hooks", "g.sh")); err != nil {
		t.Fatal(err)
	}
	res := Reconcile(claude, false)
	if res.Err != nil || len(res.Removed) != 1 {
		t.Fatalf("%+v", res)
	}
	if _, ok := readJSON(t, filepath.Join(repo, ".claude", "settings.json"))["hooks"]; ok {
		t.Error("empty hooks key should be removed")
	}
}

func TestReconcileCopilot(t *testing.T) {
	root := t.TempDir()
	tg := config.Target{Name: "copilot", Agent: "copilot", Path: filepath.Join(root, ".copilot")}
	script := filepath.Join(tg.Path, "hooks", "guard.sh")
	writeFile(t, script, "#!/bin/sh\n# agentcfg-hook: PreToolUse Bash timeout=10\n")
	file := filepath.Join(tg.Path, "hooks", "agentcfg.json")

	if res := Reconcile(tg, false); res.Err != nil || len(res.Added) != 1 || !res.Written {
		t.Fatalf("%+v", res)
	}
	var f copilotFile
	raw, _ := os.ReadFile(file)
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	want := copilotHook{Type: "command", Matcher: "Bash", Bash: script, TimeoutSec: 10}
	if f.Version != 1 || len(f.Hooks["PreToolUse"]) != 1 || f.Hooks["PreToolUse"][0] != want {
		t.Fatalf("got %s", raw)
	}
	if res := Reconcile(tg, false); res.Written {
		t.Error("unchanged file was rewritten")
	}

	if err := os.Remove(script); err != nil {
		t.Fatal(err)
	}
	if res := Reconcile(tg, false); res.Err != nil || len(res.Removed) != 1 {
		t.Fatalf("%+v", res)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Error("empty Copilot file should be deleted")
	}
}
