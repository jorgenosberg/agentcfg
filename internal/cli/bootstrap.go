package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/source"
	"github.com/jorgenosberg/agentcfg/internal/wizard"
)

func newInitCmd(pathFlag *string) *cobra.Command {
	var sourcePath string
	var noInteractive bool
	c := &cobra.Command{
		Use:   "init",
		Short: "Write a default config file",
		Long: "Write a starter config at the configured path (default " +
			"~/.agentcfg/config.json) and create the source tree skeleton.\n\n" +
			"When stdin is a terminal the command launches an interactive wizard " +
			"that discovers installed agents, lets you register targets, and " +
			"optionally imports items into the source tree. Pass --no-interactive " +
			"to skip the wizard and write a bare config file.",
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

			// Launch interactive wizard when stdin is a real terminal.
			fi, statErr := os.Stdin.Stat()
			isTTY := statErr == nil && (fi.Mode()&os.ModeCharDevice) != 0
			if isTTY && !noInteractive {
				return wizard.RunInit(path, sourcePath)
			}

			// Non-interactive: write a bare default config.
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
	c.Flags().BoolVar(&noInteractive, "no-interactive", false, "write a bare config file without the setup wizard")
	return c
}
