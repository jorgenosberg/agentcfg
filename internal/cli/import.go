package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/source"
	"github.com/jorgenosberg/agentcfg/internal/sync"
)

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

			items, err := source.ScanWith(t.Path, t.SupportedSubdirs())
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
