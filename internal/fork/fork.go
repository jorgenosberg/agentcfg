package fork

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jorgenosberg/agentcfg/internal/claudecfg"
	"github.com/jorgenosberg/agentcfg/internal/forks"
	"github.com/jorgenosberg/agentcfg/internal/marketplace"
	"github.com/jorgenosberg/agentcfg/internal/plugins"
	isync "github.com/jorgenosberg/agentcfg/internal/sync"
)

// Request describes a fork operation.
type Request struct {
	Plugin                plugins.Plugin
	ForksRoot             string // ~/.agentcfg/forks (the fork marketplace root)
	ForksPath             string // path to forks.json
	SettingsPath          string // path to <claude-dir>/settings.json
	KnownMarketplacesPath string // path to <claude-dir>/plugins/known_marketplaces.json
	InstalledPluginsPath  string // path to <claude-dir>/plugins/installed_plugins.json
	ClaudeDir             string // the Claude Code directory being registered (e.g. ~/.claude)
}

// Result reports the outcome of a fork.
type Result struct {
	BundlePath   string // absolute path of the copied bundle
	ForkFullName string // "<name>@agentcfg-forks"
}

// Execute performs the fork: copies the entire plugin bundle into the agentcfg
// fork marketplace, registers it with Claude Code, disables the upstream plugin,
// enables the fork, and records provenance in forks.json.
//
// A plugin already forked for one Claude Code directory can be forked again
// for another: the bundle is copied only once and reused, while the
// marketplace/settings registration always runs for req.ClaudeDir. Forking
// the same plugin into the same ClaudeDir twice is rejected.
func Execute(req Request) (Result, error) {
	p := req.Plugin
	bundleDest := marketplace.BundlePath(req.ForksRoot, p.Name)
	forkFullName := marketplace.ForkFullName(p.Name)

	ff, err := forks.Load(req.ForksPath)
	if err != nil {
		return Result{}, fmt.Errorf("load forks: %w", err)
	}

	existing, alreadyTracked := ff.Forks[p.FullName]
	if alreadyTracked && existing.HasClaudeDir(req.ClaudeDir) {
		return Result{}, fmt.Errorf("plugin %q is already forked for %q", p.FullName, req.ClaudeDir)
	}

	_, statErr := os.Stat(bundleDest)
	bundleExists := statErr == nil
	if bundleExists && !alreadyTracked {
		// Bundle present on disk but forks.json has no record of it — an
		// orphaned or foreign directory. Refuse to clobber it.
		return Result{}, fmt.Errorf("fork already exists at %s; remove it first", bundleDest)
	}

	if !bundleExists {
		// Create the plugins/ directory; CopyAny creates the bundle dir itself.
		if err := os.MkdirAll(filepath.Dir(bundleDest), 0o755); err != nil {
			return Result{}, fmt.Errorf("create fork plugins dir: %w", err)
		}
		// Copy the full bundle verbatim.
		if err := isync.CopyAny(p.InstallPath, bundleDest); err != nil {
			return Result{}, fmt.Errorf("copy plugin bundle: %w", err)
		}
		// Register the plugin in the agentcfg fork marketplace manifest.
		if err := marketplace.AddPlugin(req.ForksRoot, p); err != nil {
			return Result{}, fmt.Errorf("update fork marketplace manifest: %w", err)
		}
	}

	now := time.Now().UTC()

	// Register the fork marketplace with Claude Code for this ClaudeDir.
	if err := marketplace.RegisterMarketplace(req.KnownMarketplacesPath, req.ForksRoot); err != nil {
		return Result{}, fmt.Errorf("register fork marketplace: %w", err)
	}

	// Add the fork to Claude Code's installed plugins registry for this ClaudeDir.
	if err := marketplace.RegisterInstalled(req.InstalledPluginsPath, p, bundleDest, now); err != nil {
		return Result{}, fmt.Errorf("register fork in installed_plugins: %w", err)
	}

	// Enable the fork in Claude Code settings for this ClaudeDir.
	if err := claudecfg.SetPluginEnabled(req.SettingsPath, forkFullName, true); err != nil {
		return Result{}, fmt.Errorf("enable fork in settings: %w", err)
	}

	// Disable the upstream plugin for this ClaudeDir.
	if err := claudecfg.SetPluginEnabled(req.SettingsPath, p.FullName, false); err != nil {
		return Result{}, fmt.Errorf("disable upstream in settings: %w", err)
	}

	// Record provenance in forks.json, preserving the original fork time and
	// source version, and appending this ClaudeDir to the tracked list.
	forkedAt := now
	sourceVersion := p.GitCommitSha
	dirs := []string{req.ClaudeDir}
	if alreadyTracked {
		forkedAt = existing.ForkedAt
		sourceVersion = existing.SourceVersion
		dirs = append(existing.ClaudeDirs, req.ClaudeDir)
	}
	ff.Forks[p.FullName] = forks.Fork{
		ForkedAt:         forkedAt,
		SourceVersion:    sourceVersion,
		UpstreamFullName: p.FullName,
		ForkFullName:     forkFullName,
		BundlePath:       bundleDest,
		UpstreamDisabled: true,
		ClaudeDirs:       dirs,
	}
	if err := forks.Save(req.ForksPath, ff); err != nil {
		return Result{}, fmt.Errorf("save forks: %w", err)
	}

	return Result{BundlePath: bundleDest, ForkFullName: forkFullName}, nil
}
