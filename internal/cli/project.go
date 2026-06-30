package cli

import (
	"fmt"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/source"
)

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
			name := projectName
			if len(args) == 1 {
				name = args[0]
			}
			if name != "" {
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
