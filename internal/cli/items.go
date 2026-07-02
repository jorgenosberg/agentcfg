package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/source"
	"github.com/jorgenosberg/agentcfg/internal/sync"
)

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
			return writeItems(cmd.OutOrStdout(), items, cfg.Source)
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
	var force bool
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
			targets, err := resolveTargets(cfg, targetName)
			if err != nil {
				return err
			}
			install := sync.Install
			if force {
				install = sync.Adopt
			}
			var failed int
			for _, t := range targets {
				st, err := install(t, t.ResolveStrategy(cfg.DefaultStrategy), item)
				if err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\terror: %v\n", t.Name, item.Name, err)
					failed++
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", t.Name, item.Name, st)
			}
			if failed > 0 {
				return fmt.Errorf("%d target(s) failed", failed)
			}
			return nil
		},
	}
	c.Flags().StringVarP(&targetName, "target", "t", "", "target name (default: all)")
	c.Flags().BoolVar(&force, "force", false, "adopt unmanaged files (replace existing files not managed by agentcfg)")
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
			targets, err := resolveTargets(cfg, targetName)
			if err != nil {
				return err
			}
			var failed int
			for _, t := range targets {
				if err := sync.Uninstall(t, t.ResolveStrategy(cfg.DefaultStrategy), item); err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\terror: %v\n", t.Name, item.Name, err)
					failed++
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\tremoved\n", t.Name, item.Name)
			}
			if failed > 0 {
				return fmt.Errorf("%d target(s) failed", failed)
			}
			return nil
		},
	}
	c.Flags().StringVarP(&targetName, "target", "t", "", "target name (default: all)")
	return c
}

func newToggleCmd(load func() (config.Config, error), pathOf func() (string, error)) *cobra.Command {
	var targetName string
	var forceOn, forceOff bool
	c := &cobra.Command{
		Use:   "toggle <item>",
		Short: "Enable or disable an item for one or more targets",
		Long: "Toggle the disabled state of an item. Without --on or --off, the direction " +
			"is inferred: if the item is disabled on all specified targets it is enabled; " +
			"otherwise it is disabled. When targets have mixed state, use -t or --on/--off.",
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
			items, err := source.Scan(cfg.Source)
			if err != nil {
				return err
			}
			item, ok := findItem(items, args[0])
			if !ok {
				return fmt.Errorf("item %q not found in %s", args[0], cfg.Source)
			}
			targets, err := resolveTargets(cfg, targetName)
			if err != nil {
				return err
			}

			disable := forceOff
			if !forceOn && !forceOff {
				allDisabled := true
				for _, t := range targets {
					if !t.IsDisabled(item) {
						allDisabled = false
						break
					}
				}
				if allDisabled {
					disable = false
				} else {
					anyDisabled := false
					for _, t := range targets {
						if t.IsDisabled(item) {
							anyDisabled = true
							break
						}
					}
					if anyDisabled && len(targets) > 1 {
						return fmt.Errorf("mixed disabled state across targets; use -t or --on/--off to be explicit")
					}
					disable = true
				}
			}

			verb := "disabled"
			if !disable {
				verb = "enabled"
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			var failed int
			for _, t := range targets {
				if err := sync.Toggle(cfgPath, t.Name, item, disable); err != nil {
					fmt.Fprintf(tw, "%s\t%s\terror: %v\n", t.Name, item.Name, err)
					failed++
					continue
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", t.Name, item.Name, verb)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			if failed > 0 {
				return fmt.Errorf("%d target(s) failed", failed)
			}
			return nil
		},
	}
	c.Flags().StringVarP(&targetName, "target", "t", "", "target name (default: all)")
	c.Flags().BoolVar(&forceOn, "on", false, "enable the item")
	c.Flags().BoolVar(&forceOff, "off", false, "disable the item")
	return c
}

func newUnmanageCmd(load func() (config.Config, error), pathOf func() (string, error)) *cobra.Command {
	var targetName string
	c := &cobra.Command{
		Use:   "unmanage <item>",
		Short: "Return an item to the target dir as a real file and stop managing it",
		Long: "Removes the symlink (or managed copy) at the destination and writes a real " +
			"copy of the file from source. The item remains in the agentcfg source tree " +
			"and is added to the target's disabled list so future syncs leave it alone.",
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
			items, err := source.Scan(cfg.Source)
			if err != nil {
				return err
			}
			item, ok := findItem(items, args[0])
			if !ok {
				return fmt.Errorf("item %q not found in %s", args[0], cfg.Source)
			}
			targets, err := resolveTargets(cfg, targetName)
			if err != nil {
				return err
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			var failed int
			for _, t := range targets {
				if err := sync.Unmanage(t, t.ResolveStrategy(cfg.DefaultStrategy), item); err != nil {
					fmt.Fprintf(tw, "%s\t%s\terror: %v\n", t.Name, item.Name, err)
					failed++
					continue
				}
				if err := sync.Toggle(cfgPath, t.Name, item, true); err != nil {
					fmt.Fprintf(tw, "%s\t%s\tunmanaged (disable failed: %v)\n", t.Name, item.Name, err)
					failed++
					continue
				}
				fmt.Fprintf(tw, "%s\t%s\tunmanaged\n", t.Name, item.Name)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			if failed > 0 {
				return fmt.Errorf("%d target(s) failed", failed)
			}
			return nil
		},
	}
	c.Flags().StringVarP(&targetName, "target", "t", "", "target name (default: all)")
	return c
}
