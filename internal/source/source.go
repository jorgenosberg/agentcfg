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
}

// Scan walks a tree using DefaultSubdirs.
func Scan(root string) ([]Item, error) {
	return ScanWith(root, DefaultSubdirs)
}

// ScanWith walks a tree using the provided layout.
//
// Rules:
//   - skill: subdir must be non-empty; entries are directories under it.
//   - hook:  subdir must be non-empty; entries are files under it.
//   - context: subdir may be empty (scan root, .md files only) or non-empty
//     (scan that dir, all non-hidden files).
func ScanWith(root string, sd Subdirs) ([]Item, error) {
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat root: %w", err)
	}

	var items []Item

	if sub, ok := sd[KindSkill]; ok && sub != "" {
		dir := filepath.Join(root, sub)
		entries, err := os.ReadDir(dir)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("read %s: %w", dir, err)
		}
		for _, e := range entries {
			if hidden(e.Name()) {
				continue
			}
			p := filepath.Join(dir, e.Name())
			fi, err := os.Stat(p) // follow symlinks
			if err != nil || !fi.IsDir() {
				continue
			}
			items = append(items, Item{
				Kind: KindSkill,
				Name: e.Name(),
				Path: p,
			})
		}
	}

	if sub, ok := sd[KindHook]; ok && sub != "" {
		dir := filepath.Join(root, sub)
		entries, err := os.ReadDir(dir)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("read %s: %w", dir, err)
		}
		for _, e := range entries {
			if hidden(e.Name()) {
				continue
			}
			p := filepath.Join(dir, e.Name())
			fi, err := os.Stat(p)
			if err != nil || fi.IsDir() {
				continue
			}
			items = append(items, Item{
				Kind: KindHook,
				Name: e.Name(),
				Path: p,
			})
		}
	}

	if sub, ok := sd[KindContext]; ok {
		dir := filepath.Join(root, sub)
		entries, err := os.ReadDir(dir)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("read %s: %w", dir, err)
		}
		rootScan := sub == ""
		for _, e := range entries {
			if e.IsDir() || hidden(e.Name()) {
				continue
			}
			if rootScan && !isMarkdown(e.Name()) {
				continue
			}
			items = append(items, Item{
				Kind: KindContext,
				Name: e.Name(),
				Path: filepath.Join(dir, e.Name()),
			})
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
