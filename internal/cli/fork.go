package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/jorgenosberg/agentcfg/internal/claudecfg"
	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/fork"
	"github.com/jorgenosberg/agentcfg/internal/forks"
	"github.com/jorgenosberg/agentcfg/internal/paths"
	"github.com/jorgenosberg/agentcfg/internal/plugins"
)

// newForkCmd returns the "fork" command group.
//
// Usage:
//
//	agentcfg fork <plugin@marketplace>   # fork a plugin
//	agentcfg fork list                   # list recorded forks
//	agentcfg fork status                 # check drift vs upstream
func newForkCmd(load func() (config.Config, error)) *cobra.Command {
	var skillNames []string
	var full bool
	var dryRun bool

	c := &cobra.Command{
		Use:   "fork <plugin@marketplace>",
		Short: "Fork a Claude plugin into the agentcfg source tree",
		Long: "fork copies a Claude Code plugin's skills and hooks into the " +
			"agentcfg source tree, records provenance, and disables the " +
			"upstream plugin. Forked items are then managed like any other " +
			"source item.\n\n" +
			"Pass --skill <name> one or more times to fork individual skills " +
			"only. Pass --full to fork all skills and hooks (default when no " +
			"--skill is given).\n\n" +
			"Use the `list` and `status` subcommands to inspect recorded forks.",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			fullName := args[0]

			cfg, err := load()
			if err != nil {
				return err
			}

			reg, err := plugins.Load()
			if err != nil {
				return fmt.Errorf("load plugins: %w", err)
			}
			plugin, ok := reg.Get(fullName)
			if !ok {
				return fmt.Errorf("plugin %q not found in Claude Code plugin registry", fullName)
			}
			if !plugin.Installed {
				return fmt.Errorf("plugin %q is not installed", fullName)
			}

			scope := fork.ScopeSkill
			if full || len(skillNames) == 0 {
				scope = fork.ScopeFull
			}

			forksPath, err := defaultForksPath()
			if err != nil {
				return err
			}
			settingsPath, err := claudecfg.DefaultPath()
			if err != nil {
				return fmt.Errorf("resolve settings path: %w", err)
			}

			req := fork.Request{
				Plugin:       plugin,
				Scope:        scope,
				Skills:       skillNames,
				SourceRoot:   cfg.Source,
				ForksPath:    forksPath,
				SettingsPath: settingsPath,
			}

			if dryRun {
				skills, hooks := resolveDryRunComponents(req)
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would fork %q\n", fullName)
				fmt.Fprintf(cmd.OutOrStdout(), "  skills: %s\n", joinOrNone(skills))
				fmt.Fprintf(cmd.OutOrStdout(), "  hooks:  %s\n", joinOrNone(hooks))
				if len(plugin.MCPServers)+len(plugin.LSPServers) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "  skipped (cannot file-fork): mcp=%v lsp=%v\n",
						plugin.MCPServers, plugin.LSPServers)
				}
				return nil
			}

			res, err := fork.Execute(req)
			if err != nil && len(res.ForkedSkills) == 0 && len(res.ForkedHooks) == 0 {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "forked %q\n", fullName)
			fmt.Fprintf(cmd.OutOrStdout(), "  skills: %s\n", joinOrNone(res.ForkedSkills))
			fmt.Fprintf(cmd.OutOrStdout(), "  hooks:  %s\n", joinOrNone(res.ForkedHooks))
			if len(res.Skipped.MCPServers)+len(res.Skipped.LSPServers) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  skipped (cannot file-fork): mcp=%v lsp=%v\n",
					res.Skipped.MCPServers, res.Skipped.LSPServers)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "  plugin disabled in settings")
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  warning: %v\n", err)
			}
			return nil
		},
	}

	c.Flags().StringArrayVar(&skillNames, "skill", nil, "fork only this skill (repeatable)")
	c.Flags().BoolVar(&full, "full", false, "fork all skills and hooks (default when no --skill given)")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be forked without making changes")

	c.AddCommand(
		newForkListCmd(),
		newForkStatusCmd(),
	)
	return c
}

// newForkListCmd is "fork list".
func newForkListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List recorded plugin forks",
		RunE: func(cmd *cobra.Command, args []string) error {
			forksPath, err := defaultForksPath()
			if err != nil {
				return err
			}
			ff, err := forks.Load(forksPath)
			if err != nil {
				return fmt.Errorf("load forks: %w", err)
			}
			if len(ff.Forks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no forks recorded")
				return nil
			}
			return writeForkList(cmd.OutOrStdout(), ff)
		},
	}
}

// newForkStatusCmd is "fork status": compares the recorded SourceVersion to the
// currently installed plugin version (no network needed).
func newForkStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check whether upstream plugins have advanced past the forked version",
		RunE: func(cmd *cobra.Command, args []string) error {
			forksPath, err := defaultForksPath()
			if err != nil {
				return err
			}
			ff, err := forks.Load(forksPath)
			if err != nil {
				return fmt.Errorf("load forks: %w", err)
			}
			if len(ff.Forks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no forks recorded")
				return nil
			}

			reg, err := plugins.Load()
			if err != nil {
				return fmt.Errorf("load plugins: %w", err)
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "PLUGIN\tFORKED AT\tFORKED SHA\tCURRENT SHA\tSTATUS")
			for fullName, f := range ff.Forks {
				currentSHA := "(not installed)"
				driftStatus := "plugin not installed"
				if p, ok := reg.Get(fullName); ok {
					currentSHA = shortSHA(p.GitCommitSha)
					switch {
					case p.GitCommitSha == "" || f.SourceVersion == "":
						driftStatus = "unknown"
					case p.GitCommitSha == f.SourceVersion:
						driftStatus = "up-to-date"
					default:
						driftStatus = "upstream advanced"
					}
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
					fullName,
					f.ForkedAt.Format(time.DateOnly),
					shortSHA(f.SourceVersion),
					currentSHA,
					driftStatus,
				)
			}
			return tw.Flush()
		},
	}
}

// helpers

func defaultForksPath() (string, error) {
	home, err := paths.Home()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, ".agentcfg", "forks.json"), nil
}

func resolveDryRunComponents(req fork.Request) (skills, hooks []string) {
	switch req.Scope {
	case fork.ScopeFull:
		return req.Plugin.Skills, req.Plugin.Hooks
	default:
		return req.Skills, nil
	}
}

func joinOrNone(ss []string) string {
	if len(ss) == 0 {
		return "(none)"
	}
	return strings.Join(ss, ", ")
}

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

func writeForkList(w io.Writer, ff *forks.ForkFile) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PLUGIN\tFORKED AT\tSHA\tSKILLS\tHOOKS")
	for fullName, f := range ff.Forks {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			fullName,
			f.ForkedAt.Format(time.DateOnly),
			shortSHA(f.SourceVersion),
			joinOrNone(f.Skills),
			joinOrNone(f.Hooks),
		)
	}
	return tw.Flush()
}
