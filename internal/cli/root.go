package cli

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

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
		Long: "agentcfg keeps a single ~/.ai source of truth in sync with one or more " +
			"AI coding agent directories (Claude Code, Codex, Copilot, opencode, ...).",
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
	)
	return root
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
			"~/.agentcfg/config.json). Use --source to point at an existing " +
			"source tree (skills/, hooks/, context/); otherwise the default " +
			"~/.agentcfg/source/ is used and the user is expected to populate " +
			"it (or symlink it to their own convention).",
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
			if err := os.MkdirAll(cfg.Source, 0o755); err != nil {
				return fmt.Errorf("create source dir: %w", err)
			}
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s (source: %s)\n", path, cfg.Source)
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
