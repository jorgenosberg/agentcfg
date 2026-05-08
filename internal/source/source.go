package source

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Kind enumerates the categories of items agentcfg syncs.
const (
	KindSkill   = "skill"
	KindHook    = "hook"
	KindContext = "context"
)

// Item is one syncable entry in the source tree.
type Item struct {
	Kind string // KindSkill | KindHook | KindContext
	Name string // basename without extension for files, dir name for skills
	Path string // absolute path in source tree
}

// Scan walks a source root and returns all items.
// Layout:
//
//	<root>/skills/<name>/        -> skill items (directories)
//	<root>/hooks/<name>.<ext>    -> hook items (files)
//	<root>/context/<name>.<ext>  -> context items (files)
func Scan(root string) ([]Item, error) {
	var items []Item

	skillsDir := filepath.Join(root, "skills")
	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() || isHidden(e.Name()) {
				continue
			}
			items = append(items, Item{
				Kind: KindSkill,
				Name: e.Name(),
				Path: filepath.Join(skillsDir, e.Name()),
			})
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read skills: %w", err)
	}

	for _, kind := range []string{KindHook, KindContext} {
		dir := filepath.Join(root, kind+"s")
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", kind, err)
		}
		for _, e := range entries {
			if e.IsDir() || isHidden(e.Name()) {
				continue
			}
			items = append(items, Item{
				Kind: kind,
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

func isHidden(name string) bool {
	return len(name) > 0 && name[0] == '.'
}
