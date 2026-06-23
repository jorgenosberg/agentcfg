package fork

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jorgenosberg/agentcfg/internal/claudecfg"
	"github.com/jorgenosberg/agentcfg/internal/forks"
	"github.com/jorgenosberg/agentcfg/internal/plugins"
	isync "github.com/jorgenosberg/agentcfg/internal/sync"
)

// Scope controls how much of a plugin is forked.
type Scope int

const (
	// ScopeSkill forks only the named skills.
	ScopeSkill Scope = iota
	// ScopeFull forks all forkable components (skills + hooks).
	ScopeFull
)

// Request describes a fork operation.
type Request struct {
	Plugin       plugins.Plugin
	Scope        Scope
	Skills       []string // used when Scope == ScopeSkill
	SourceRoot   string   // agentcfg source root (e.g. ~/.agentcfg/source)
	ForksPath    string   // path to forks.json
	SettingsPath string   // path to ~/.claude/settings.json
}

// Result reports what was forked and what was skipped.
type Result struct {
	ForkedSkills []string
	ForkedHooks  []string
	Skipped      forks.Skipped
}

// Execute performs the fork: copies files, records provenance, disables the plugin.
func Execute(req Request) (Result, error) {
	skillsToCopy, hooksToCopy := resolveComponents(req)

	var res Result
	var copyErrs []error

	skillsDir := filepath.Join(req.SourceRoot, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create skills dir: %w", err)
	}

	for _, name := range skillsToCopy {
		src := filepath.Join(req.Plugin.InstallPath, "skills", name)
		dst := filepath.Join(skillsDir, name)
		if _, err := os.Stat(dst); err == nil {
			copyErrs = append(copyErrs, fmt.Errorf("skill %q already exists in source; remove it first or rename the fork", name))
			continue
		}
		if err := isync.CopyAny(src, dst); err != nil {
			copyErrs = append(copyErrs, fmt.Errorf("copy skill %q: %w", name, err))
			continue
		}
		res.ForkedSkills = append(res.ForkedSkills, name)
	}

	hooksDir := filepath.Join(req.SourceRoot, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create hooks dir: %w", err)
	}

	for _, name := range hooksToCopy {
		src := filepath.Join(req.Plugin.InstallPath, "hooks", name)
		dst := filepath.Join(hooksDir, name)
		if _, err := os.Stat(dst); err == nil {
			copyErrs = append(copyErrs, fmt.Errorf("hook %q already exists in source; remove it first or rename the fork", name))
			continue
		}
		if err := isync.CopyAny(src, dst); err != nil {
			copyErrs = append(copyErrs, fmt.Errorf("copy hook %q: %w", name, err))
			continue
		}
		res.ForkedHooks = append(res.ForkedHooks, name)
	}

	if len(copyErrs) > 0 && len(res.ForkedSkills) == 0 && len(res.ForkedHooks) == 0 {
		return Result{}, fmt.Errorf("fork failed: %w", copyErrs[0])
	}

	// Record non-forkable components.
	res.Skipped = forks.Skipped{
		MCPServers: req.Plugin.MCPServers,
		LSPServers: req.Plugin.LSPServers,
	}

	// Write forks.json.
	ff, err := forks.Load(req.ForksPath)
	if err != nil {
		return res, fmt.Errorf("load forks: %w", err)
	}
	ff.Forks[req.Plugin.FullName] = forks.Fork{
		ForkedAt:       time.Now().UTC(),
		SourceVersion:  req.Plugin.GitCommitSha,
		PluginDisabled: true,
		Skills:         res.ForkedSkills,
		Hooks:          res.ForkedHooks,
		Skipped:        res.Skipped,
	}
	if err := forks.Save(req.ForksPath, ff); err != nil {
		return res, fmt.Errorf("save forks: %w", err)
	}

	// Disable the original plugin.
	if err := claudecfg.SetPluginEnabled(req.SettingsPath, req.Plugin.FullName, false); err != nil {
		return res, fmt.Errorf("disable plugin in settings: %w", err)
	}

	// Surface non-fatal copy errors as a combined error on partial success.
	if len(copyErrs) > 0 {
		var b strings.Builder
		for _, e := range copyErrs {
			b.WriteString("\n  ")
			b.WriteString(e.Error())
		}
		return res, fmt.Errorf("partial fork (some components skipped):%s", b.String())
	}

	return res, nil
}

func resolveComponents(req Request) (skills, hooks []string) {
	switch req.Scope {
	case ScopeFull:
		return req.Plugin.Skills, req.Plugin.Hooks
	default: // ScopeSkill
		return req.Skills, nil
	}
}
