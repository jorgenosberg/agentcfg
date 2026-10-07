package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/tui"
)

func newTUICmd(resolveCfg func() (config.Config, error), resolvePath func() (string, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open the interactive TUI (default when run with no arguments in a terminal)",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runTUI(resolveCfg, resolvePath)
		},
	}
}

func runTUI(resolveCfg func() (config.Config, error), resolvePath func() (string, error)) error {
	cfg, err := resolveCfg()
	if err != nil {
		return err
	}
	path, err := resolvePath()
	if err != nil {
		return err
	}
	return tui.Run(path, cfg)
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
