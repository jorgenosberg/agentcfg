package cli

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/jorgenosberg/agentcfg/internal/claudecfg"
	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/fork"
	"github.com/jorgenosberg/agentcfg/internal/forks"
	"github.com/jorgenosberg/agentcfg/internal/marketplace"
	"github.com/jorgenosberg/agentcfg/internal/plugins"
)

// newForkCmd returns the "fork" command group.
//
// Usage:
//
//	agentcfg fork <plugin@marketplace>   # fork a plugin
//	agentcfg fork list                   # list recorded forks
//	agentcfg fork status                 # check drift vs upstream
func newForkCmd(_ func() (config.Config, error)) *cobra.Command { //nolint:unparam
	var dryRun bool

	c := &cobra.Command{
		Use:   "fork <plugin@marketplace>",
		Short: "Fork a Claude plugin into the agentcfg-owned marketplace",
		Long: "fork copies a Claude Code plugin's entire bundle into the agentcfg " +
			"fork marketplace (~/.agentcfg/forks/), registers it with Claude Code, " +
			"disables the upstream plugin, and enables the fork. You own the copy " +
			"and can edit any file in it directly.\n\n" +
			"Use the `list` and `status` subcommands to inspect recorded forks.",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			fullName := args[0]

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

			forksRoot, err := marketplace.DefaultForksRoot()
			if err != nil {
				return fmt.Errorf("resolve forks root: %w", err)
			}
			forksPath, err := forks.DefaultPath()
			if err != nil {
				return err
			}
			settingsPath, err := claudecfg.DefaultPath()
			if err != nil {
				return fmt.Errorf("resolve settings path: %w", err)
			}
			knownMPPath, err := marketplace.DefaultKnownMarketplacesPath()
			if err != nil {
				return fmt.Errorf("resolve known_marketplaces path: %w", err)
			}
			installedPath, err := marketplace.DefaultInstalledPluginsPath()
			if err != nil {
				return fmt.Errorf("resolve installed_plugins path: %w", err)
			}

			if dryRun {
				bundleDest := marketplace.BundlePath(forksRoot, plugin.Name)
				forkFullName := marketplace.ForkFullName(plugin.Name)
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would fork %q\n", fullName)
				fmt.Fprintf(cmd.OutOrStdout(), "  bundle source:  %s\n", plugin.InstallPath)
				fmt.Fprintf(cmd.OutOrStdout(), "  bundle dest:    %s\n", bundleDest)
				fmt.Fprintf(cmd.OutOrStdout(), "  fork identity:  %s\n", forkFullName)
				fmt.Fprintf(cmd.OutOrStdout(), "  enable:         %s\n", forkFullName)
				fmt.Fprintf(cmd.OutOrStdout(), "  disable:        %s\n", fullName)
				return nil
			}

			req := fork.Request{
				Plugin:                plugin,
				ForksRoot:             forksRoot,
				ForksPath:             forksPath,
				SettingsPath:          settingsPath,
				KnownMarketplacesPath: knownMPPath,
				InstalledPluginsPath:  installedPath,
			}

			res, err := fork.Execute(req)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "forked %q\n", fullName)
			fmt.Fprintf(cmd.OutOrStdout(), "  bundle: %s\n", res.BundlePath)
			fmt.Fprintf(cmd.OutOrStdout(), "  fork:   %s (enabled)\n", res.ForkFullName)
			fmt.Fprintf(cmd.OutOrStdout(), "  upstream %s disabled\n", fullName)
			return nil
		},
	}

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
			forksPath, err := forks.DefaultPath()
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
// currently installed upstream plugin version (no network needed).
func newForkStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check whether upstream plugins have advanced past the forked version",
		RunE: func(cmd *cobra.Command, args []string) error {
			forksPath, err := forks.DefaultPath()
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
			for upstreamName, f := range ff.Forks {
				currentSHA := "(not installed)"
				driftStatus := "plugin not installed"
				if p, ok := reg.Get(upstreamName); ok {
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
					upstreamName,
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

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

func writeForkList(w io.Writer, ff *forks.ForkFile) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PLUGIN\tFORKED AT\tSHA\tBUNDLE")
	for upstreamName, f := range ff.Forks {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			upstreamName,
			f.ForkedAt.Format(time.DateOnly),
			shortSHA(f.SourceVersion),
			f.BundlePath,
		)
	}
	return tw.Flush()
}
