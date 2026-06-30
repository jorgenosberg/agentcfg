package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jorgenosberg/agentcfg/internal/catalog"
	"github.com/jorgenosberg/agentcfg/internal/config"
)

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
				fmt.Fprintln(tw, "NAME\tAGENT\tALIAS\tPATH\tSTRATEGY")
				for _, t := range cfg.Targets {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
						t.Name, t.Agent, t.Alias, t.Path, t.ResolveStrategy(cfg.DefaultStrategy))
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
	var strategy, agentType, alias string
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
			t := catalog.TargetFor(agentType, args[1], name)
			t.Strategy = strategy
			if alias != "" {
				t.Alias = alias
			}
			cfg.Targets = append(cfg.Targets, t)
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
	c.Flags().StringVar(&agentType, "agent", "", "agent type profile: claude, codex, copilot, gemini, cursor, cline, windsurf, aider, agents, opencode")
	c.Flags().StringVar(&alias, "alias", "", "group alias (e.g. 'claude' to group multiple Claude targets together)")
	return c
}
