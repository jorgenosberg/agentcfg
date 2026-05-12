// Package wizard provides interactive TUI setup flows built on charmbracelet/huh.
package wizard

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/jorgenosberg/agentcfg/internal/catalog"
	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/source"
	"github.com/jorgenosberg/agentcfg/internal/sync"
)

// RunInit runs the interactive first-time setup wizard, writing a config to
// cfgPath. defaultSource is pre-filled into the source path input (empty →
// config.DefaultSource() is used).
func RunInit(cfgPath, defaultSource string) error {
	src := defaultSource
	if src == "" {
		s, err := config.DefaultSource()
		if err != nil {
			return err
		}
		src = s
	}

	// ── Step 1: source tree path ──────────────────────────────────────────
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Source tree path").
				Description("agentcfg syncs from this directory into all configured targets.").
				Value(&src),
		),
	).Run(); err != nil {
		return abort(err)
	}

	// ── Step 2: discover agents → select sync targets ─────────────────────
	found := catalog.Discover()
	var selectedTargetNames []string
	if len(found) > 0 {
		opts := make([]huh.Option[string], len(found))
		for i, t := range found {
			opts[i] = huh.NewOption(fmt.Sprintf("%-10s  %s", t.Name, t.Path), t.Name)
		}
		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Sync targets").
					Description("Select agent directories to register (space to toggle, a for all).").
					Options(opts...).
					Value(&selectedTargetNames),
			),
		).Run(); err != nil {
			return abort(err)
		}
	} else {
		fmt.Println("  no known agent directories found — you can add targets later with `agentcfg target add`")
	}

	selectedTargets := targetsForNames(found, selectedTargetNames)

	// ── Step 3: scan items in selected targets → choose what to import ────
	// Collect every item found across all selected targets, then deduplicate:
	// items with the same kind+name and identical content are shown once;
	// items with the same kind+name but different content are shown with the
	// originating agent name as a disambiguator.
	type rawCandidate struct {
		target config.Target
		item   source.Item
		hash   string
	}
	var raw []rawCandidate
	for _, t := range selectedTargets {
		items, err := source.ScanWith(t.Path, t.Subdirs)
		if err != nil {
			continue
		}
		for _, it := range items {
			h, err := contentHash(it.Path)
			if err != nil {
				h = t.Name + ":" + it.Path // can't hash → treat as unique
			}
			raw = append(raw, rawCandidate{target: t, item: it, hash: h})
		}
	}

	// Group by kind/name; within each group collect distinct hashes in order.
	type hashGroup struct {
		entries []rawCandidate
		hashes  map[string]bool
	}
	groups := map[string]*hashGroup{}
	var groupOrder []string
	for _, rc := range raw {
		k := rc.item.Kind + "/" + rc.item.Name
		if _, exists := groups[k]; !exists {
			groups[k] = &hashGroup{hashes: map[string]bool{}}
			groupOrder = append(groupOrder, k)
		}
		g := groups[k]
		if !g.hashes[rc.hash] {
			g.hashes[rc.hash] = true
			g.entries = append(g.entries, rc)
		}
	}

	// Flatten into the final deduplicated candidate list.
	type candidate struct {
		target config.Target
		item   source.Item
		key    string
		label  string
	}
	var candidates []candidate
	for _, k := range groupOrder {
		g := groups[k]
		multiContent := len(g.hashes) > 1
		for _, rc := range g.entries {
			key := rc.item.Kind + "/" + rc.item.Name
			label := fmt.Sprintf("%-8s  %s", rc.item.Kind, rc.item.Name)
			if multiContent {
				// Same name, different content — show which agent it came from.
				key = rc.target.Name + "/" + rc.item.Kind + "/" + rc.item.Name
				label = fmt.Sprintf("%-8s  %-24s  [%s]", rc.item.Kind, rc.item.Name, rc.target.Name)
			}
			candidates = append(candidates, candidate{
				target: rc.target, item: rc.item, key: key, label: label,
			})
		}
	}

	var selectedKeys []string
	if len(candidates) > 0 {
		opts := make([]huh.Option[string], len(candidates))
		for i, c := range candidates {
			opts[i] = huh.NewOption(c.label, c.key)
		}
		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Import items").
					Description("Items to copy into your source tree now (space to toggle, a for all).").
					Options(opts...).
					Value(&selectedKeys),
			),
		).Run(); err != nil {
			return abort(err)
		}
	}

	// ── Step 4: optionally add a project folder ───────────────────────────
	var addProject bool
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Add a project folder?").
				Description("Project folders are repos/workspaces agentcfg scans for in-repo agent config (CLAUDE.md, .cursorrules, etc.).").
				Value(&addProject),
		),
	).Run(); err != nil {
		return abort(err)
	}

	var projName, projPath string
	if addProject {
		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Project name").
					Description("A short label, e.g. \"myapp\"").
					Value(&projName),
				huh.NewInput().
					Title("Project path").
					Description("Absolute or relative path to the repository root.").
					Value(&projPath),
			),
		).Run(); err != nil {
			return abort(err)
		}
	}

	// ── Apply ─────────────────────────────────────────────────────────────

	for _, sub := range source.DefaultSubdirs {
		if sub == "" {
			continue
		}
		if err := os.MkdirAll(filepath.Join(src, sub), 0o755); err != nil {
			return fmt.Errorf("create source subdir: %w", err)
		}
	}

	cfg := config.Default(src)
	cfg.Targets = selectedTargets

	if addProject && projName != "" && projPath != "" {
		abs, err := filepath.Abs(projPath)
		if err != nil {
			abs = projPath
		}
		cfg.Projects = append(cfg.Projects, config.Project{Name: projName, Path: abs})
	}

	if err := config.Save(cfgPath, cfg); err != nil {
		return err
	}

	// Import selected items
	toImport := map[string]bool{}
	for _, k := range selectedKeys {
		toImport[k] = true
	}
	imported := 0
	for _, c := range candidates {
		if !toImport[c.key] {
			continue
		}
		destSub := source.DefaultSubdirs[c.item.Kind]
		destDir := filepath.Join(src, destSub)
		dest := filepath.Join(destDir, c.item.Name)
		if _, err := os.Lstat(dest); err == nil {
			continue // already exists
		}
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			continue
		}
		if err := sync.CopyAny(c.item.Path, dest); err != nil {
			fmt.Fprintf(os.Stderr, "  warn: import %s: %v\n", c.item.Name, err)
			continue
		}
		imported++
	}

	// ── Summary ───────────────────────────────────────────────────────────
	fmt.Println()
	fmt.Printf("  config   → %s\n", cfgPath)
	fmt.Printf("  source   → %s\n", src)
	if len(selectedTargets) > 0 {
		names := make([]string, len(selectedTargets))
		for i, t := range selectedTargets {
			names[i] = t.Name
		}
		fmt.Printf("  targets  → %s\n", strings.Join(names, ", "))
	}
	if imported > 0 {
		fmt.Printf("  imported → %d item(s)\n", imported)
	}
	if addProject && projName != "" {
		fmt.Printf("  project  → %s\n", projName)
	}
	fmt.Println()
	fmt.Println("Run `agentcfg status` to see sync state.")
	return nil
}

func targetsForNames(all []config.Target, names []string) []config.Target {
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	var out []config.Target
	for _, t := range all {
		if set[t.Name] {
			out = append(out, t)
		}
	}
	return out
}

func abort(err error) error {
	if errors.Is(err, huh.ErrUserAborted) {
		return fmt.Errorf("aborted")
	}
	return err
}

// contentHash returns a SHA-256 digest of a file or directory tree.
// For directories, the hash covers the relative path and content of every
// non-directory entry (sorted by filepath.WalkDir order) so that structural
// differences are captured. On any I/O error the returned string is empty and
// the error is propagated — callers should treat an empty hash as unique.
func contentHash(path string) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	if fi.IsDir() {
		err = filepath.WalkDir(path, func(p string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() {
				return walkErr
			}
			rel, _ := filepath.Rel(path, p)
			h.Write([]byte(rel))
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			h.Write(data)
			return nil
		})
	} else {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		h.Write(data)
	}
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
