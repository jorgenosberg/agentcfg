package cli

import (
	"fmt"
	"maps"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	agentpkg "github.com/jorgenosberg/agentcfg/internal/agent"
	"github.com/jorgenosberg/agentcfg/internal/catalog"
	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/source"
)

func newDiscoverCmd(load func() (config.Config, error), pathOf func() (string, error)) *cobra.Command {
	var addNames []string
	var addAll, showPaths bool
	var customPath, asAgent string
	c := &cobra.Command{
		Use:   "discover",
		Short: "List known AI agent install dirs and items found in them",
		Long: "Walk the built-in catalog of known agent install paths under " +
			"$HOME and list items found in each. Read-only by default. Use " +
			"--add <name> (repeatable) or --add-all to register discovered " +
			"agents as targets in the config. Use --paths to print which " +
			"paths the catalog checks without scanning.\n\n" +
			"Use --path <dir> to scan a custom directory instead of the catalog. " +
			"Supply --as <agent-type> to apply that agent's layout defaults.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if showPaths {
				tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "NAME\tPATH")
				for _, a := range catalog.KnownAgents() {
					fmt.Fprintf(tw, "%s\t%s\n", a.Name, a.Path)
				}
				return tw.Flush()
			}

			cfg, err := load()
			if err != nil {
				return err
			}

			// Custom path mode
			if customPath != "" {
				abs, err := filepath.Abs(customPath)
				if err != nil {
					return fmt.Errorf("resolve path: %w", err)
				}
				srcItems, _ := source.Scan(cfg.Source)
				haveInSource := map[string]bool{}
				for _, it := range srcItems {
					haveInSource[it.Kind+"/"+it.Name] = true
				}
				// Build effective subdirs from profile; fall back to defaults for unknown agents.
				effectiveSubdirs := source.Subdirs{}
				if asAgent != "" {
					if p, ok := agentpkg.Get(asAgent); ok {
						maps.Copy(effectiveSubdirs, p.Subdirs)
					}
				}
				if len(effectiveSubdirs) == 0 {
					effectiveSubdirs = source.DefaultSubdirs
				}
				items, err := source.ScanWith(abs, effectiveSubdirs)
				if err != nil {
					return fmt.Errorf("scan %s: %w", abs, err)
				}
				tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "TARGET\tKIND\tNAME\tIN_SOURCE")
				if len(items) == 0 {
					fmt.Fprintf(tw, "%s\t-\t(no items)\t-\n", abs)
				}
				for _, it := range items {
					mark := "no"
					if haveInSource[it.Kind+"/"+it.Name] {
						mark = "yes"
					}
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", abs, it.Kind, it.Name, mark)
				}
				tw.Flush()

				if len(addNames) == 0 && !addAll {
					return nil
				}
				if len(addNames) == 0 {
					return fmt.Errorf("--add-all requires individual target names when using --path; use --add <name>")
				}
				targetName := addNames[0]
				for _, t := range cfg.Targets {
					if t.Name == targetName {
						fmt.Fprintf(cmd.OutOrStdout(), "skip %s (already configured)\n", targetName)
						return nil
					}
				}
				newT := catalog.TargetFor(asAgent, abs, targetName)
				cfg.Targets = append(cfg.Targets, newT)
				fmt.Fprintf(cmd.OutOrStdout(), "added %s -> %s\n", targetName, abs)
				p, err := pathOf()
				if err != nil {
					return err
				}
				return config.Save(p, cfg)
			}

			// Standard catalog mode
			found := catalog.Discover()
			if len(found) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no known agent directories found in $HOME")
				fmt.Fprintln(cmd.OutOrStdout(), "run `agentcfg discover --paths` to see which paths are checked")
				return nil
			}

			srcItems, _ := source.Scan(cfg.Source)
			haveInSource := map[string]bool{}
			for _, it := range srcItems {
				haveInSource[it.Kind+"/"+it.Name] = true
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "TARGET\tKIND\tNAME\tIN_SOURCE")
			for _, t := range found {
				items, err := source.ScanWith(t.Path, t.Subdirs)
				if err != nil {
					fmt.Fprintf(tw, "%s\t-\t-\terror: %v\n", t.Name, err)
					continue
				}
				if len(items) == 0 {
					fmt.Fprintf(tw, "%s\t-\t(no items)\t-\n", t.Name)
					continue
				}
				for _, it := range items {
					mark := "no"
					if haveInSource[it.Kind+"/"+it.Name] {
						mark = "yes"
					}
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", t.Name, it.Kind, it.Name, mark)
				}
			}
			tw.Flush()

			if !addAll && len(addNames) == 0 {
				return nil
			}

			selected := selectForAdd(found, addNames, addAll)
			if len(selected) == 0 {
				return fmt.Errorf("nothing to add (none of %v matched discovered agents)", addNames)
			}
			existing := map[string]bool{}
			for _, t := range cfg.Targets {
				existing[t.Name] = true
			}
			added := 0
			for _, t := range selected {
				if existing[t.Name] {
					fmt.Fprintf(cmd.OutOrStdout(), "skip %s (already configured)\n", t.Name)
					continue
				}
				cfg.Targets = append(cfg.Targets, t)
				fmt.Fprintf(cmd.OutOrStdout(), "added %s -> %s\n", t.Name, t.Path)
				added++
			}
			if added == 0 {
				return nil
			}
			p, err := pathOf()
			if err != nil {
				return err
			}
			return config.Save(p, cfg)
		},
	}
	c.Flags().StringSliceVar(&addNames, "add", nil, "register named discovered agent as target (repeatable)")
	c.Flags().BoolVar(&addAll, "add-all", false, "register every discovered agent as a target")
	c.Flags().BoolVar(&showPaths, "paths", false, "print catalog paths without scanning, then exit")
	c.Flags().StringVar(&customPath, "path", "", "scan this directory instead of the built-in catalog")
	c.Flags().StringVar(&asAgent, "as", "", "agent type for --path (claude, codex, etc.)")
	return c
}

func selectForAdd(found []config.Target, names []string, all bool) []config.Target {
	if all {
		return found
	}
	wanted := map[string]bool{}
	for _, n := range names {
		wanted[n] = true
	}
	var out []config.Target
	for _, t := range found {
		if wanted[t.Name] {
			out = append(out, t)
		}
	}
	return out
}
