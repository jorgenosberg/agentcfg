package source

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	KindSkill   = "skill"
	KindHook    = "hook"
	KindContext = "context"
	KindCommand = "command" // agent-specific slash commands / prompt files
	KindRule    = "rule"    // agent-specific rule files (.cursorrules, etc.)
)

// Item is one syncable entry in a tree (source or target).
type Item struct {
	Kind string
	Name string
	Path string
}

// Subdirs maps Kind -> subdirectory under the tree root.
// An empty string means: scan the root itself for items of that kind.
// A missing kind means: do not scan for items of that kind.
type Subdirs map[string]string

// DefaultSubdirs is the layout used by an agentcfg source tree.
var DefaultSubdirs = Subdirs{
	KindSkill:   "skills",
	KindHook:    "hooks",
	KindContext: "context",
	KindCommand: "commands",
	KindRule:    "rules",
}

// kindDesc describes how a Kind is scanned within ScanWith.
type kindDesc struct {
	isDir       bool // entries are directories (skills) or files (everything else)
	needsSubdir bool // skip when subdir is "" (hooks and commands require an explicit subdir)
	mdOnly      bool // when scanning root (subdir=""), limit to .md/.markdown files (context)
	includeDots bool // include dotfile entries (rule files like .cursorrules are dotfiles)
}

// kindDescs maps each known Kind to its scan descriptor.
// Unknown kinds appearing in a Subdirs map are silently skipped.
var kindDescs = map[string]kindDesc{
	KindSkill:   {isDir: true, needsSubdir: true},
	KindHook:    {isDir: false, needsSubdir: true},
	KindContext: {isDir: false, mdOnly: true},
	KindCommand: {isDir: false, needsSubdir: true},
	KindRule:    {isDir: false, includeDots: true},
}

// Scan walks a tree using DefaultSubdirs.
func Scan(root string) ([]Item, error) {
	return ScanWith(root, DefaultSubdirs)
}

// ScanWith walks a tree using the provided layout.
//
// Rules per Kind:
//   - skill:   subdir required; entries are directories.
//   - hook:    subdir required; entries are files.
//   - context: subdir may be empty (scan root, .md files only) or a named dir.
//   - command: subdir required; entries are files.
//   - rule:    subdir may be empty (scan root, all non-hidden files) or a named dir.
func ScanWith(root string, sd Subdirs) ([]Item, error) {
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat root: %w", err)
	}

	var items []Item

	for kind, sub := range sd {
		desc, known := kindDescs[kind]
		if !known {
			continue
		}
		if desc.needsSubdir && sub == "" {
			continue
		}

		dir := filepath.Join(root, sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read %s: %w", dir, err)
		}

		rootScan := sub == ""
		for _, e := range entries {
			if !desc.includeDots && hidden(e.Name()) {
				continue
			}
			p := filepath.Join(dir, e.Name())
			fi, err := os.Stat(p) // follow symlinks
			if err != nil {
				continue
			}
			if desc.isDir {
				if !fi.IsDir() {
					continue
				}
			} else {
				if fi.IsDir() {
					continue
				}
				if rootScan && desc.mdOnly && !isMarkdown(e.Name()) {
					continue
				}
			}
			items = append(items, Item{Kind: kind, Name: e.Name(), Path: p})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func hidden(name string) bool { return len(name) > 0 && name[0] == '.' }

func isMarkdown(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".md" || ext == ".markdown"
}

// ProjectItem is one agent-specific file or directory found inside a project
// (repository) folder rather than the global agentcfg source tree.
type ProjectItem struct {
	Project string // user-assigned project name
	Agent   string // which agent this belongs to: "claude", "copilot", etc.
	Kind    string // KindContext / KindSkill / KindHook / KindCommand / KindRule
	Name    string // display name (basename or directory entry name)
	Path    string // absolute path on disk
	RelPath string // path relative to the project root
}

// projectScanRule describes one pattern to look for inside a project root.
type projectScanRule struct {
	agent   string
	kind    string
	relPath string // path relative to project root
	isDir   bool   // if true, list non-hidden entries inside this directory
}

// projectRules is the built-in table of per-agent file patterns searched
// inside each project folder. Order determines display order.
var projectRules = []projectScanRule{
	// Claude Code
	{agent: "claude", kind: KindContext, relPath: "CLAUDE.md"},
	{agent: "claude", kind: KindSkill, relPath: ".claude/skills", isDir: true},
	{agent: "claude", kind: KindHook, relPath: ".claude/hooks", isDir: true},
	{agent: "claude", kind: KindCommand, relPath: ".claude/commands", isDir: true},

	// GitHub Copilot
	{agent: "copilot", kind: KindContext, relPath: ".github/copilot-instructions.md"},
	{agent: "copilot", kind: KindCommand, relPath: ".github/prompts", isDir: true},

	// Codex CLI / opencode / agents (shared AGENTS.md format)
	{agent: "agents", kind: KindContext, relPath: "AGENTS.md"},

	// Gemini CLI
	{agent: "gemini", kind: KindContext, relPath: "GEMINI.md"},

	// Cursor
	{agent: "cursor", kind: KindRule, relPath: ".cursorrules"},
	{agent: "cursor", kind: KindRule, relPath: ".cursor/rules", isDir: true},

	// Cline
	{agent: "cline", kind: KindRule, relPath: ".clinerules"},

	// Windsurf
	{agent: "windsurf", kind: KindRule, relPath: ".windsurfrules"},

	// Aider
	{agent: "aider", kind: KindContext, relPath: ".aider.conf.yml"},
}

// ScanProject walks a project directory and returns all agent-specific items
// found according to the built-in rule table. name is the user-assigned label
// for this project (stored in config). Non-existent paths are silently skipped.
func ScanProject(root, name string) ([]ProjectItem, error) {
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat project root %s: %w", root, err)
	}

	var items []ProjectItem
	for _, rule := range projectRules {
		abs := filepath.Join(root, rule.relPath)
		if rule.isDir {
			entries, err := os.ReadDir(abs)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("read %s: %w", abs, err)
			}
			for _, e := range entries {
				if hidden(e.Name()) {
					continue
				}
				entryAbs := filepath.Join(abs, e.Name())
				entryRel := filepath.Join(rule.relPath, e.Name())
				items = append(items, ProjectItem{
					Project: name,
					Agent:   rule.agent,
					Kind:    rule.kind,
					Name:    e.Name(),
					Path:    entryAbs,
					RelPath: entryRel,
				})
			}
		} else {
			fi, err := os.Stat(abs)
			if err != nil || fi.IsDir() {
				continue
			}
			items = append(items, ProjectItem{
				Project: name,
				Agent:   rule.agent,
				Kind:    rule.kind,
				Name:    filepath.Base(rule.relPath),
				Path:    abs,
				RelPath: rule.relPath,
			})
		}
	}
	return items, nil
}
