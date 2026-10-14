package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jorgenosberg/agentcfg/internal/backup"
	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/source"
	"github.com/jorgenosberg/agentcfg/internal/sync"
)

const globalScope = "global"

func newMoveCmd(load func() (config.Config, error), pathOf func() (string, error)) *cobra.Command {
	var to, from, kind, targetName string
	var link, force, keep, dryRun, noBackup bool
	c := &cobra.Command{
		Use:   "move <item> --to <project|global>",
		Short: "Move an item between global scope and a project",
		Long: "Moves an item between your global agent targets and a registered project " +
			"(see `agentcfg project add`).\n\n" +
			"--to <project> installs the item into the project for every agent that " +
			"has it globally, then disables it on those global targets. The item stays " +
			"in the source tree, so it can be moved back. Project installs are copies " +
			"unless --link is set, because a symlink into ~/.agentcfg breaks for " +
			"anyone else who clones the repository.\n\n" +
			"--to global imports the item from the project into the source tree, " +
			"installs it on the global targets (-t to limit), and removes the project " +
			"copies unless --keep is set.\n\n" +
			"Use --dry-run to see what would change.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			cfgPath, err := pathOf()
			if err != nil {
				return err
			}
			if to == "" {
				return fmt.Errorf("--to is required (a project name or %q)", globalScope)
			}
			if to == globalScope {
				return moveToGlobal(cmd.OutOrStdout(), cfg, cfgPath, args[0], from, kind, targetName, force, keep, dryRun, noBackup)
			}
			if from != "" && from != globalScope {
				return fmt.Errorf("moving between projects is not supported; move to %q first", globalScope)
			}
			return moveToProject(cmd.OutOrStdout(), cfg, cfgPath, args[0], to, kind, link, force, dryRun, noBackup)
		},
	}
	c.Flags().StringVar(&to, "to", "", "destination: a project name or \"global\"")
	c.Flags().StringVar(&from, "from", "", "source project when moving to global (default: the only project that has it)")
	c.Flags().StringVar(&kind, "kind", "", "item kind, to disambiguate when a name exists as several kinds")
	c.Flags().StringVarP(&targetName, "target", "t", "", "global target to install into when moving to global (default: all)")
	c.Flags().BoolVar(&link, "link", false, "symlink into the project instead of copying")
	c.Flags().BoolVar(&force, "force", false, "replace existing content at the destination")
	c.Flags().BoolVar(&keep, "keep", false, "when moving to global, leave the project copies in place")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "show what would change without changing anything")
	c.Flags().BoolVar(&noBackup, "no-backup", false, "skip the automatic backup")
	return c
}

func moveToProject(w io.Writer, cfg config.Config, cfgPath, name, projectName, kind string, link, force, dryRun, noBackup bool) error {
	p, ok := config.LookupProject(cfg, projectName)
	if !ok {
		return fmt.Errorf("project %q not configured; add it with `agentcfg project add %s <path>`", projectName, projectName)
	}
	items, err := source.Scan(cfg.Source)
	if err != nil {
		return err
	}
	it, err := pickItem(items, name, kind)
	if err != nil {
		return err
	}
	plan, err := sync.PlanToProject(cfg, it, p, link)
	if err != nil {
		return err
	}
	if err := plan.Preflight(force); err != nil {
		return err
	}
	for _, s := range plan.Installs {
		fmt.Fprintf(w, "install\t%s\n", s.Dest)
	}
	for _, s := range plan.Removals {
		fmt.Fprintf(w, "disable\t%s on %s\n", it.Name, s.Target.Name)
	}
	for _, s := range plan.Kept {
		fmt.Fprintf(w, "keep\t%s on %s (%s)\n", it.Name, s.Target.Name, s.Note)
	}
	warnHook(w, it.Kind)
	if dryRun {
		return nil
	}
	if err := backupFirst(w, cfg, noBackup); err != nil {
		return err
	}
	return plan.Apply(cfgPath, force)
}

func moveToGlobal(w io.Writer, cfg config.Config, cfgPath, name, from, kind, targetName string, force, keep, dryRun, noBackup bool) error {
	if from == globalScope {
		return fmt.Errorf("item is already global")
	}
	projects := cfg.Projects
	if from != "" {
		p, ok := config.LookupProject(cfg, from)
		if !ok {
			return fmt.Errorf("project %q not configured", from)
		}
		projects = []config.Project{p}
	}
	var found config.Project
	var copies []source.ProjectItem
	for _, p := range projects {
		scanned, err := source.ScanProject(p.Path, p.Name)
		if err != nil {
			return err
		}
		var here []source.ProjectItem
		for _, pi := range scanned {
			if pi.Name == name && (kind == "" || pi.Kind == kind) {
				here = append(here, pi)
			}
		}
		if len(here) == 0 {
			continue
		}
		if len(copies) > 0 {
			return fmt.Errorf("%q exists in projects %s and %s; pick one with --from", name, found.Name, p.Name)
		}
		found, copies = p, here
	}
	if len(copies) == 0 {
		return fmt.Errorf("%q not found in any configured project", name)
	}
	for _, c := range copies[1:] {
		if c.Kind != copies[0].Kind {
			return fmt.Errorf("%q exists as several kinds in %s; pick one with --kind", name, found.Name)
		}
	}
	targets, err := resolveTargets(cfg, targetName)
	if err != nil {
		return err
	}
	plan, err := sync.PlanToGlobal(cfg, found, copies, targets)
	if err != nil {
		return err
	}
	if err := plan.Preflight(force); err != nil {
		return err
	}
	fmt.Fprintf(w, "import\t%s\n", plan.Item.Path)
	for _, t := range plan.Targets {
		fmt.Fprintf(w, "install\t%s on %s\n", plan.Item.Name, t.Name)
	}
	if !keep {
		for _, c := range copies {
			fmt.Fprintf(w, "remove\t%s\n", c.Path)
		}
	}
	warnHook(w, plan.Item.Kind)
	if dryRun {
		return nil
	}
	if err := backupFirst(w, cfg, noBackup); err != nil {
		return err
	}
	return plan.Apply(cfgPath, cfg, force, keep)
}

func pickItem(items []source.Item, name, kind string) (source.Item, error) {
	var matches []source.Item
	for _, it := range items {
		if it.Name == name && (kind == "" || it.Kind == kind) {
			matches = append(matches, it)
		}
	}
	switch len(matches) {
	case 0:
		return source.Item{}, fmt.Errorf("item %q not found in the source tree", name)
	case 1:
		return matches[0], nil
	}
	kinds := make([]string, len(matches))
	for i, m := range matches {
		kinds[i] = m.Kind
	}
	return source.Item{}, fmt.Errorf("%q exists as several kinds (%s); pick one with --kind", name, strings.Join(kinds, ", "))
}

func backupFirst(w io.Writer, cfg config.Config, skip bool) error {
	if skip {
		return nil
	}
	root, err := backup.DefaultRoot()
	if err != nil {
		return err
	}
	dir, err := backup.Create(cfg, root)
	if err != nil {
		return fmt.Errorf("backup: %w", err)
	}
	fmt.Fprintf(w, "backup\t%s\n", dir)
	return nil
}

// warnHook flags that agentcfg moves hook scripts but not the settings.json
// entries that run them.
func warnHook(w io.Writer, kind string) {
	if kind == source.KindHook {
		fmt.Fprintln(w, "warn\thook registrations in settings.json are not moved; update the global and project settings to the new path")
	}
}
