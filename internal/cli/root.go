package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jorgenosberg/agentcfg/internal/config"
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
			"~/.agentcfg/source.\n\n" +
			"Set AGENTCFG_HOME to any directory to run in an isolated sandbox — " +
			"all reads and writes (state, catalog, Claude plugin files) go there " +
			"instead of the real $HOME.",
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
		newToggleCmd(resolveCfg, resolvePath),
		newUnmanageCmd(resolveCfg, resolvePath),
		newSyncCmd(resolveCfg),
		newBackupCmd(resolveCfg),
		newEditCmd(resolveCfg),
		newVersionCmd(resolveCfg),
		newInitCmd(&configPath),
		newTargetCmd(resolveCfg, resolvePath),
		newDiscoverCmd(resolveCfg, resolvePath),
		newImportCmd(resolveCfg),
		newProjectCmd(resolveCfg, resolvePath),
		newForkCmd(),
	)
	return root
}
