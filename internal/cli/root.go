package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jorgenosberg/agentcfg/internal/catalog"
	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/source"
	"github.com/jorgenosberg/agentcfg/internal/sync"
	"github.com/jorgenosberg/agentcfg/internal/version"
)

// NewRoot builds the cobra command tree for the agentcfg CLI.
func NewRoot() *cobra.Command {
	var configPath string

	root := &cobra.Command{
		Use:   "agentcfg",
		Short: "Sync skills, hooks, and context files across AI agent configs",
		Long: "agentcfg keeps a single source-of-truth directory in sync with one or " +
			"more AI coding agent directories (Claude Code, Codex, Copilot, " +
			"opencode, ...). Source path is user-configurable; default is " +
			"~/.agentcfg/source.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.String(),
	}

	root.PersistentFlags().StringVar(&configPath, "config", "", "path to config file (default ~/.agentcfg/config.json)")

	resolveCfg := func() (config.Config, error) {
		path := configPath
		if path == "" {
			p, err := config.DefaultPath()
			if err != nil {
				return config.Config{}, err
			}
			path = p
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return config.Config{}, fmt.Errorf(
				"no config found at %s — run `agentcfg init` to bootstrap", path)
		}
		return config.Load(path)
	}

	resolvePath := func() (string, error) {
		if configPath != "" {
			return configPath, nil
		}
		return config.DefaultPath()
	}

	root.AddCommand(
		newListCmd(resolveCfg),
		newStatusCmd(resolveCfg),
		newInstallCmd(resolveCfg),
		newUninstallCmd(resolveCfg),
		newInitCmd(&configPath),
		newTargetCmd(resolveCfg, resolvePath),
		newDiscoverCmd(resolveCfg, resolvePath),
		newImportCmd(resolveCfg),
		newProjectCmd(resolveCfg, resolvePath),
	)
	return root
}

func newDiscoverCmd(load func() (config.Config, error), pathOf func() (string, error)) *cobra.Command {
	var addNames []string
	var addAll, showPaths bool
	c := &cobra.Command{
		Use:   "discover",
		Short: "List known AI agent install dirs and items found in them",
		Long: "Walk the built-in catalog of known agent install paths under " +
			"$HOME and list items found in each. Read-only by default. Use " +
			"--add <name> (repeatable) or --add-all to register discovered " +
			"agents as targets in the config. Use --paths to print which " +
			"paths the catalog checks without scanning.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if showPaths {
				tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "NAME\tPATH")
				for _, a := range catalog.KnownAgents() {
					fmt.Fprintf(tw, "%s\t%s\n", a.Name, a.Path)
				}
				return tw.Flush()
			}

			cfg, err := load()
			if err != nil {
				return err
			}
			found := catalog.Discover()
			if len(found) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no known agent directories found in $HOME")
				fmt.Fprintln(cmd.OutOrStdout(), "run `agentcfg discover --paths` to see which paths are checked")
				return nil
			}

			srcItems, _ := source.Scan(cfg.Source)
			haveInSource := map[string]bool{}
			for _, it := range srcItems {
				haveInSource[it.Kind+"/"+it.Name] = true
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "TARGET\tKIND\tNAME\tIN_SOURCE")
			for _, t := range found {
				items, err := source.ScanWith(t.Path, t.Subdirs)
				if err != nil {
					fmt.Fprintf(tw, "%s\t-\t-\terror: %v\n", t.Name, err)
					continue
				}
				if len(items) == 0 {
					fmt.Fprintf(tw, "%s\t-\t(no items)\t-\n", t.Name)
					continue
				}
				for _, it := range items {
					mark := "no"
					if haveInSource[it.Kind+"/"+it.Name] {
						mark = "yes"
					}
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", t.Name, it.Kind, it.Name, mark)
				}
			}
			tw.Flush()

			if !addAll && len(addNames) == 0 {
				return nil
			}

			selected := selectForAdd(found, addNames, addAll)
			if len(selected) == 0 {
				return fmt.Errorf("nothing to add (none of %v matched discovered agents)", addNames)
			}
			existing := map[string]bool{}
			for _, t := range cfg.Targets {
				existing[t.Name] = true
			}
			added := 0
			for _, t := range selected {
				if existing[t.Name] {
					fmt.Fprintf(cmd.OutOrStdout(), "skip %s (already configured)\n", t.Name)
					continue
				}
				cfg.Targets = append(cfg.Targets, t)
				fmt.Fprintf(cmd.OutOrStdout(), "added %s -> %s\n", t.Name, t.Path)
				added++
			}
			if added == 0 {
				return nil
			}
			p, err := pathOf()
			if err != nil {
				return err
			}
			return config.Save(p, cfg)
		},
	}
	c.Flags().StringSliceVar(&addNames, "add", nil, "register named discovered agent as target (repeatable)")
	c.Flags().BoolVar(&addAll, "add-all", false, "register every discovered agent as a target")
	c.Flags().BoolVar(&showPaths, "paths", false, "print catalog paths without scanning, then exit")
	return c
}

func selectForAdd(found []config.Target, names []string, all bool) []config.Target {
	if all {
		return found
	}
	wanted := map[string]bool{}
	for _, n := range names {
		wanted[n] = true
	}
	var out []config.Target
	for _, t := range found {
		if wanted[t.Name] {
			out = append(out, t)
		}
	}
	return out
}

func newImportCmd(load func() (config.Config, error)) *cobra.Command {
	var all, force bool
	c := &cobra.Command{
		Use:   "import <target> [item...]",
		Short: "Copy items from a target's directory into the source tree",
		Long: "Reads the given target's directory and copies named items " +
			"into the source tree. Use --all to import everything found. " +
			"Existing source items are skipped unless --force is set.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			targetName := args[0]
			wanted := args[1:]
			if !all && len(wanted) == 0 {
				return fmt.Errorf("specify item names or pass --all")
			}

			t, ok := lookupTarget(cfg, targetName)
			if !ok {
				return fmt.Errorf("target %q not configured; run `agentcfg discover` or `agentcfg target add`", targetName)
			}

			items, err := source.ScanWith(t.Path, t.Subdirs)
			if err != nil {
				return err
			}
			selected := items
			if !all {
				selected = nil
				for _, w := range wanted {
					found := false
					for _, it := range items {
						if it.Name == w {
							selected = append(selected, it)
							found = true
							break
						}
					}
					if !found {
						return fmt.Errorf("item %q not found under %s", w, t.Path)
					}
				}
			}

			for _, it := range selected {
				destSub := source.DefaultSubdirs[it.Kind]
				destDir := filepath.Join(cfg.Source, destSub)
				dest := filepath.Join(destDir, it.Name)
				if _, err := os.Lstat(dest); err == nil && !force {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\tskip (exists)\n", it.Kind, it.Name)
					continue
				}
				if err := os.MkdirAll(destDir, 0o755); err != nil {
					return err
				}
				if force {
					_ = os.RemoveAll(dest)
				}
				if err := sync.CopyAny(it.Path, dest); err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\terror: %v\n", it.Kind, it.Name, err)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\timported\n", it.Kind, it.Name)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&all, "all", false, "import every item found in the target")
	c.Flags().BoolVar(&force, "force", false, "overwrite source items that already exist")
	return c
}

func lookupTarget(cfg config.Config, name string) (config.Target, bool) {
	for _, t := range cfg.Targets {
		if t.Name == name {
			return t, true
		}
	}
	return config.Target{}, false
}

func newProjectCmd(load func() (config.Config, error), pathOf func() (string, error)) *cobra.Command {
	c := &cobra.Command{
		Use:   "project",
		Short: "Manage project folders to scan for in-repo agent configuration",
		Long: "Project folders are repository or workspace directories that " +
			"agentcfg scans for agent-specific files such as CLAUDE.md, " +
			".github/copilot-instructions.md, .claude/skills/, .cursorrules, " +
			"and similar artefacts. Scanning is read-only; use `import` to " +
			"pull items into the global source tree.",
	}
	c.AddCommand(
		newProjectListCmd(load),
		newProjectAddCmd(load, pathOf),
		newProjectRemoveCmd(load, pathOf),
		newProjectScanCmd(load),
	)
	return c
}

func newProjectListCmd(load func() (config.Config, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured project folders",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tPATH")
			for _, p := range cfg.Projects {
				fmt.Fprintf(tw, "%s\t%s\n", p.Name, p.Path)
			}
			return tw.Flush()
		},
	}
}

func newProjectAddCmd(load func() (config.Config, error), pathOf func() (string, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <path>",
		Short: "Add a project folder",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			name, path := args[0], args[1]
			abs, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("resolve path: %w", err)
			}
			for _, p := range cfg.Projects {
				if p.Name == name {
					return fmt.Errorf("project %q already exists", name)
				}
			}
			cfg.Projects = append(cfg.Projects, config.Project{Name: name, Path: abs})
			cfgPath, err := pathOf()
			if err != nil {
				return err
			}
			if err := config.Save(cfgPath, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added project %s -> %s\n", name, abs)
			return nil
		},
	}
}

func newProjectRemoveCmd(load func() (config.Config, error), pathOf func() (string, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a project folder",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			name := args[0]
			out := cfg.Projects[:0]
			removed := false
			for _, p := range cfg.Projects {
				if p.Name == name {
					removed = true
					continue
				}
				out = append(out, p)
			}
			if !removed {
				return fmt.Errorf("project %q not found", name)
			}
			cfg.Projects = out
			cfgPath, err := pathOf()
			if err != nil {
				return err
			}
			if err := config.Save(cfgPath, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed project %s\n", name)
			return nil
		},
	}
}

func newProjectScanCmd(load func() (config.Config, error)) *cobra.Command {
	var projectName string
	c := &cobra.Command{
		Use:   "scan [name]",
		Short: "Scan project folder(s) for in-repo agent configuration files",
		Long: "Walks configured project directories and lists all agent-specific " +
			"files and directories found (CLAUDE.md, .github/copilot-instructions.md, " +
			".claude/skills/, .cursorrules, etc.). Provide a project name to scan " +
			"only that project. Read-only.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			if len(cfg.Projects) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no projects configured")
				fmt.Fprintln(cmd.OutOrStdout(), "add one with: agentcfg project add <name> <path>")
				return nil
			}

			projects := cfg.Projects
			if len(args) == 1 {
				name := args[0]
				found := false
				for _, p := range cfg.Projects {
					if p.Name == name {
						projects = []config.Project{p}
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("project %q not configured", name)
				}
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "PROJECT\tAGENT\tKIND\tNAME\tREL_PATH")
			anyFound := false
			for _, p := range projects {
				items, err := source.ScanProject(p.Path, p.Name)
				if err != nil {
					fmt.Fprintf(tw, "%s\t-\t-\terror\t%v\n", p.Name, err)
					continue
				}
				if len(items) == 0 {
					fmt.Fprintf(tw, "%s\t-\t(no items found)\t-\t-\n", p.Name)
					continue
				}
				anyFound = true
				for _, it := range items {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
						it.Project, it.Agent, it.Kind, it.Name, it.RelPath)
				}
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			if !anyFound && len(projects) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nno agent configuration files found in configured projects")
			}
			return nil
		},
	}
	c.Flags().StringVarP(&projectName, "project", "p", "", "project name (default: all)")
	return c
}

func newTargetCmd(load func() (config.Config, error), pathOf func() (string, error)) *cobra.Command {
	c := &cobra.Command{
		Use:   "target",
		Short: "Manage sync targets",
	}
	c.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List configured targets",
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, err := load()
				if err != nil {
					return err
				}
				tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "NAME\tPATH\tSTRATEGY")
				for _, t := range cfg.Targets {
					fmt.Fprintf(tw, "%s\t%s\t%s\n", t.Name, t.Path, t.ResolveStrategy(cfg.DefaultStrategy))
				}
				return tw.Flush()
			},
		},
		newTargetAddCmd(load, pathOf),
		&cobra.Command{
			Use:   "remove <name>",
			Short: "Remove a target",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, err := load()
				if err != nil {
					return err
				}
				name := args[0]
				out := cfg.Targets[:0]
				removed := false
				for _, t := range cfg.Targets {
					if t.Name == name {
						removed = true
						continue
					}
					out = append(out, t)
				}
				if !removed {
					return fmt.Errorf("target %q not found", name)
				}
				cfg.Targets = out
				p, err := pathOf()
				if err != nil {
					return err
				}
				if err := config.Save(p, cfg); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", name)
				return nil
			},
		},
	)
	return c
}

func newTargetAddCmd(load func() (config.Config, error), pathOf func() (string, error)) *cobra.Command {
	var strategy string
	c := &cobra.Command{
		Use:   "add <name> <path>",
		Short: "Add a sync target",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			name := args[0]
			for _, t := range cfg.Targets {
				if t.Name == name {
					return fmt.Errorf("target %q already exists", name)
				}
			}
			cfg.Targets = append(cfg.Targets, config.Target{
				Name:     name,
				Path:     args[1],
				Strategy: strategy,
			})
			p, err := pathOf()
			if err != nil {
				return err
			}
			if err := config.Save(p, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added %s -> %s\n", name, args[1])
			return nil
		},
	}
	c.Flags().StringVar(&strategy, "strategy", "", "link or copy (default: config default)")
	return c
}

func newListCmd(load func() (config.Config, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List items in the source tree",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			items, err := source.Scan(cfg.Source)
			if err != nil {
				return err
			}
			return writeItems(cmd.OutOrStdout(), items)
		},
	}
}

func newStatusCmd(load func() (config.Config, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show install state of every item across every target",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			items, err := source.Scan(cfg.Source)
			if err != nil {
				return err
			}
			entries := sync.Inspect(cfg, items)
			return writeStatus(cmd.OutOrStdout(), entries)
		},
	}
}

func newInstallCmd(load func() (config.Config, error)) *cobra.Command {
	var targetName string
	c := &cobra.Command{
		Use:   "install <item>",
		Short: "Install an item into one or more targets",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			items, err := source.Scan(cfg.Source)
			if err != nil {
				return err
			}
			item, ok := findItem(items, args[0])
			if !ok {
				return fmt.Errorf("item %q not found in %s", args[0], cfg.Source)
			}
			targets := selectTargets(cfg.Targets, targetName)
			if len(targets) == 0 {
				return fmt.Errorf("no matching targets")
			}
			for _, t := range targets {
				st, err := sync.Install(t, t.ResolveStrategy(cfg.DefaultStrategy), item)
				if err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\terror: %v\n", t.Name, item.Name, err)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", t.Name, item.Name, st)
			}
			return nil
		},
	}
	c.Flags().StringVarP(&targetName, "target", "t", "", "target name (default: all)")
	return c
}

func newUninstallCmd(load func() (config.Config, error)) *cobra.Command {
	var targetName string
	c := &cobra.Command{
		Use:   "uninstall <item>",
		Short: "Remove an item from one or more targets",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			items, err := source.Scan(cfg.Source)
			if err != nil {
				return err
			}
			item, ok := findItem(items, args[0])
			if !ok {
				return fmt.Errorf("item %q not found in %s", args[0], cfg.Source)
			}
			for _, t := range selectTargets(cfg.Targets, targetName) {
				if err := sync.Uninstall(t, t.ResolveStrategy(cfg.DefaultStrategy), item); err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\terror: %v\n", t.Name, item.Name, err)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\tremoved\n", t.Name, item.Name)
			}
			return nil
		},
	}
	c.Flags().StringVarP(&targetName, "target", "t", "", "target name (default: all)")
	return c
}

func newInitCmd(pathFlag *string) *cobra.Command {
	var sourcePath string
	c := &cobra.Command{
		Use:   "init",
		Short: "Write a default config file",
		Long: "Write a starter config at the configured path (default " +
			"~/.agentcfg/config.json) and create the source tree skeleton. " +
			"No target directories are scanned or registered — run " +
			"`agentcfg discover` to see known agents and `--add` to register " +
			"them.",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := *pathFlag
			if path == "" {
				p, err := config.DefaultPath()
				if err != nil {
					return err
				}
				path = p
			}
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("config already exists at %s", path)
			}
			cfg := config.Default(sourcePath)
			for _, sub := range source.DefaultSubdirs {
				if sub == "" {
					continue
				}
				if err := os.MkdirAll(filepath.Join(cfg.Source, sub), 0o755); err != nil {
					return fmt.Errorf("create source subdir: %w", err)
				}
			}
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", path)
			fmt.Fprintf(cmd.OutOrStdout(), "  source: %s\n", cfg.Source)
			fmt.Fprintln(cmd.OutOrStdout(), "next: run `agentcfg discover` to find AI agent dirs in $HOME")
			return nil
		},
	}
	c.Flags().StringVar(&sourcePath, "source", "", "path to source tree (default ~/.agentcfg/source)")
	return c
}

func writeItems(w io.Writer, items []source.Item) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "KIND\tNAME\tPATH")
	for _, it := range items {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", it.Kind, it.Name, it.Path)
	}
	return tw.Flush()
}

func writeStatus(w io.Writer, entries []sync.Entry) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TARGET\tKIND\tITEM\tSTATUS\tDEST")
	for _, e := range entries {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			e.Target.Name, e.Item.Kind, e.Item.Name, e.Status, e.Dest)
	}
	return tw.Flush()
}

func findItem(items []source.Item, name string) (source.Item, bool) {
	for _, it := range items {
		if it.Name == name {
			return it, true
		}
	}
	return source.Item{}, false
}

func selectTargets(all []config.Target, name string) []config.Target {
	if name == "" {
		return all
	}
	for _, t := range all {
		if t.Name == name {
			return []config.Target{t}
		}
	}
	return nil
}
