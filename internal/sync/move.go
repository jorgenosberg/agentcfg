package sync

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/source"
)

// MoveStep is one install or removal performed by a move.
type MoveStep struct {
	Target config.Target
	Dest   string
	Note   string
}

// ProjectMovePlan describes moving an item between global targets and a project.
type ProjectMovePlan struct {
	Item     source.Item
	Installs []MoveStep
	Removals []MoveStep
	Kept     []MoveStep // global targets left enabled; Note says why
}

// PlanToProject plans installing it into project p for every agent that has
// it installed globally, then disabling it on those global targets. Targets
// whose agent has no project-level location for the kind stay enabled, so
// they keep the item. Dest paths are deduplicated, since several agents read
// the same project dir.
func PlanToProject(cfg config.Config, it source.Item, p config.Project, link bool) (ProjectMovePlan, error) {
	if err := movable(it.Kind); err != nil {
		return ProjectMovePlan{}, err
	}
	plan := ProjectMovePlan{Item: it}
	seen := map[string]bool{}
	for _, t := range cfg.Targets {
		st := statusOf(destPath(t, it), it.Path, t.ResolveStrategy(cfg.DefaultStrategy))
		if t.IsDisabled(it) || !t.SupportsKind(it.Kind) || st == StatusAbsent || st == StatusUnmanaged {
			continue
		}
		pt, ok := config.ProjectTarget(p, t.Agent, link)
		if ok {
			_, ok = pt.Subdirs[it.Kind]
		}
		if !ok {
			plan.Kept = append(plan.Kept, MoveStep{Target: t, Note: fmt.Sprintf("%s has no project-level %s location", t.Agent, it.Kind)})
			continue
		}
		plan.Removals = append(plan.Removals, MoveStep{Target: t, Dest: destPath(t, it)})
		dest := destPath(pt, it)
		if seen[dest] {
			continue
		}
		seen[dest] = true
		plan.Installs = append(plan.Installs, MoveStep{Target: pt, Dest: dest})
	}
	if len(plan.Removals) == 0 {
		if len(plan.Kept) > 0 {
			return plan, fmt.Errorf("no agent has a project-level location for %s %s", it.Kind, it.Name)
		}
		return plan, fmt.Errorf("%s is not installed on any global target", it.Name)
	}
	return plan, nil
}

// Preflight reports conflicts that would make Apply fail, without changing anything.
func (pl ProjectMovePlan) Preflight(force bool) error {
	for _, s := range pl.Installs {
		if force {
			continue
		}
		if st := statusOf(s.Dest, pl.Item.Path, s.Target.ResolveStrategy("")); st == StatusUnmanaged || st == StatusDrifted {
			return fmt.Errorf("%s already exists with different content (use --force to replace)", s.Dest)
		}
	}
	return nil
}

// Apply runs the plan. Global targets are only disabled after every project
// install succeeded, so a failure never leaves the item installed nowhere.
// Existing project files with different content are kept unless force is set.
func (pl ProjectMovePlan) Apply(cfgPath string, force bool) error {
	if err := pl.Preflight(force); err != nil {
		return err
	}
	for _, s := range pl.Installs {
		strategy := s.Target.ResolveStrategy("")
		if _, err := Adopt(s.Target, strategy, pl.Item); err != nil {
			return fmt.Errorf("install into %s: %w", s.Dest, err)
		}
	}
	for _, s := range pl.Removals {
		if err := Toggle(cfgPath, s.Target.Name, pl.Item, true); err != nil {
			return err
		}
	}
	return nil
}

// GlobalMovePlan describes moving an item from a project into the source
// tree and the global targets.
type GlobalMovePlan struct {
	Project config.Project
	Item    source.Item // the item as it will exist in the source tree
	Copies  []source.ProjectItem
	Targets []config.Target
}

// PlanToGlobal plans promoting the project copies in copies to global scope.
// All copies must have identical content. targets are the global targets to
// install into.
func PlanToGlobal(cfg config.Config, p config.Project, copies []source.ProjectItem, targets []config.Target) (GlobalMovePlan, error) {
	if len(copies) == 0 {
		return GlobalMovePlan{}, fmt.Errorf("item not found in project %s", p.Name)
	}
	first := copies[0]
	if err := movable(first.Kind); err != nil {
		return GlobalMovePlan{}, err
	}
	for _, c := range copies[1:] {
		if !sameContent(first.Path, c.Path) {
			return GlobalMovePlan{}, fmt.Errorf("%s and %s have different content; resolve them first", first.RelPath, c.RelPath)
		}
	}
	it := source.Item{
		Kind: first.Kind,
		Name: first.Name,
		Path: filepath.Join(cfg.Source, source.DefaultSubdirs[first.Kind], first.Name),
	}
	var usable []config.Target
	for _, t := range targets {
		if t.SupportsKind(it.Kind) && !t.Excludes(it) {
			usable = append(usable, t)
		}
	}
	if len(usable) == 0 {
		return GlobalMovePlan{}, fmt.Errorf("no global target supports %s items", it.Kind)
	}
	return GlobalMovePlan{Project: p, Item: it, Copies: copies, Targets: usable}, nil
}

// Preflight reports conflicts that would make Apply fail, without changing anything.
func (pl GlobalMovePlan) Preflight(force bool) error {
	if _, err := os.Lstat(pl.Item.Path); err == nil && !force && !sameContent(pl.Copies[0].Path, pl.Item.Path) {
		return fmt.Errorf("source already has %s %s with different content (use --force to replace)", pl.Item.Kind, pl.Item.Name)
	}
	return nil
}

// Apply imports the project copy into the source tree, installs it on the
// global targets, then removes the project copies. Removal is last, so the
// content always exists somewhere. An existing source item with different
// content is kept unless force is set. keep leaves the project copies alone.
func (pl GlobalMovePlan) Apply(cfgPath string, cfg config.Config, force, keep bool) error {
	if err := pl.Preflight(force); err != nil {
		return err
	}
	first := pl.Copies[0]
	// A --link project copy points at the source item itself; importing it
	// with force would delete the source before copying from it.
	if !sameContent(first.Path, pl.Item.Path) {
		if _, err := ImportItem(cfg.Source, source.Item{Kind: first.Kind, Name: first.Name, Path: first.Path}, force); err != nil {
			return fmt.Errorf("import into source: %w", err)
		}
	}
	for _, t := range pl.Targets {
		if t.IsDisabled(pl.Item) {
			if err := Toggle(cfgPath, t.Name, pl.Item, false); err != nil {
				return err
			}
			continue
		}
		if _, err := Install(t, t.ResolveStrategy(cfg.DefaultStrategy), pl.Item); err != nil {
			return fmt.Errorf("install into %s: %w", t.Name, err)
		}
	}
	if keep {
		return nil
	}
	for _, c := range pl.Copies {
		if err := os.RemoveAll(c.Path); err != nil {
			return fmt.Errorf("remove %s: %w", c.Path, err)
		}
	}
	return nil
}

// movable rejects kinds that move does not handle. Context files share fixed
// names (AGENTS.md, CLAUDE.md) with the project's own, so moving them would
// overwrite one side.
func movable(kind string) error {
	if kind == source.KindContext {
		return fmt.Errorf("context files cannot be moved; edit the project's own context file instead")
	}
	return nil
}
