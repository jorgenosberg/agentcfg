package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jorgenosberg/agentcfg/internal/backup"
	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/lock"
	"github.com/jorgenosberg/agentcfg/internal/source"
	"github.com/jorgenosberg/agentcfg/internal/sync"
)

func newSyncCmd(load func() (config.Config, error)) *cobra.Command {
	var dryRun bool
	var noBackup bool
	var force bool
	var targetName string
	c := &cobra.Command{
		Use:   "sync",
		Short: "Install all absent and drifted items across all targets",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			if targetName != "" {
				targets := selectTargets(cfg.Targets, targetName)
				if len(targets) == 0 {
					return fmt.Errorf("no target named %q", targetName)
				}
				cfg.Targets = targets
			}
			if len(cfg.Targets) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no targets configured; run `agentcfg discover` to find agent dirs, then `agentcfg discover --add <name>`")
				return nil
			}
			items, err := source.Scan(cfg.Source)
			if err != nil {
				return err
			}
			lockPath, err := lock.DefaultPath()
			if err != nil {
				return err
			}
			lck, err := lock.Load(lockPath)
			if err != nil {
				return err
			}

			if !dryRun && !noBackup {
				backupRoot, err := backup.DefaultRoot()
				if err != nil {
					return err
				}
				snaps, err := backup.List(backupRoot)
				if err != nil {
					return err
				}
				// Collect names covered by any existing snapshot.
				covered := make(map[string]bool)
				for _, snap := range snaps {
					for _, ts := range snap.Targets {
						covered[ts.Name] = true
					}
				}
				// Backup if any existing target dir has never been snapshotted.
				for _, t := range cfg.Targets {
					if _, serr := os.Stat(t.Path); serr == nil && !covered[t.Name] {
						backupDir, berr := backup.Create(cfg, backupRoot)
						if berr != nil {
							return fmt.Errorf("auto-backup failed: %w", berr)
						}
						fmt.Fprintf(cmd.OutOrStdout(), "backup created: %s\n", backupDir)
						break
					}
				}
			}

			results := sync.Sync(cfg, items, lck, dryRun, force)

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			if dryRun {
				fmt.Fprintln(tw, "TARGET\tKIND\tITEM\tSTATUS\t(dry-run)")
			} else {
				fmt.Fprintln(tw, "TARGET\tKIND\tITEM\tRESULT")
			}
			for _, r := range results {
				if r.Err != nil {
					fmt.Fprintf(tw, "%s\t%s\t%s\terror: %v\n",
						r.Entry.Target.Name, r.Entry.Item.Kind, r.Entry.Item.Name, r.Err)
					continue
				}
				if dryRun {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
						r.Entry.Target.Name, r.Entry.Item.Kind, r.Entry.Item.Name, r.OldStatus)
				} else {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
						r.Entry.Target.Name, r.Entry.Item.Kind, r.Entry.Item.Name, r.Entry.Status)
				}
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			var failed int
			for _, r := range results {
				if r.Err != nil {
					failed++
				}
			}

			if !dryRun && len(results) > 0 {
				if err := lock.Save(lockPath, lck); err != nil {
					return fmt.Errorf("save lock: %w", err)
				}
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "everything up to date")
			}
			if failed > 0 {
				return fmt.Errorf("%d item(s) failed", failed)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be installed without making changes")
	c.Flags().BoolVar(&noBackup, "no-backup", false, "skip automatic backup before syncing")
	c.Flags().BoolVar(&force, "force", false, "adopt unmanaged files (replace existing files not managed by agentcfg)")
	c.Flags().StringVarP(&targetName, "target", "t", "", "target name (default: all)")
	return c
}

func newEditCmd(load func() (config.Config, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <item>",
		Short: "Open a source item in your editor",
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
			for _, it := range items {
				if it.Name == args[0] {
					return openInEditor(it.Path)
				}
			}
			return fmt.Errorf("item %q not found in source", args[0])
		},
	}
}

func openInEditor(path string) error {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	c := exec.Command(editor, path)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func newVersionCmd(load func() (config.Config, error)) *cobra.Command {
	findItemByName := func(cfg config.Config, name string) (source.Item, error) {
		items, err := source.Scan(cfg.Source)
		if err != nil {
			return source.Item{}, err
		}
		for _, it := range items {
			if it.Name == name {
				return it, nil
			}
		}
		return source.Item{}, fmt.Errorf("item %q not found in source", name)
	}

	c := &cobra.Command{
		Use:   "version",
		Short: "Manage saved versions of source items",
	}

	list := &cobra.Command{
		Use:   "list <item>",
		Short: "List saved versions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			it, err := findItemByName(cfg, args[0])
			if err != nil {
				return err
			}
			versions, err := source.ListVersions(it.Path)
			if err != nil {
				return err
			}
			if len(versions) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no saved versions")
				return nil
			}
			for _, v := range versions {
				fmt.Fprintln(cmd.OutOrStdout(), v)
			}
			return nil
		},
	}

	save := &cobra.Command{
		Use:   "save <item> <name>",
		Short: "Save the current item as a named version",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			it, err := findItemByName(cfg, args[0])
			if err != nil {
				return err
			}
			if err := source.SaveVersion(it.Path, args[1]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved %q as version %q\n", args[0], args[1])
			return nil
		},
	}

	switchCmd := &cobra.Command{
		Use:   "switch <item> <name>",
		Short: "Switch the active item to a saved version",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			it, err := findItemByName(cfg, args[0])
			if err != nil {
				return err
			}
			if err := source.SwitchVersion(it.Path, args[1]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "switched %q to version %q\n", args[0], args[1])
			return nil
		},
	}

	deleteCmd := &cobra.Command{
		Use:   "delete <item> <name>",
		Short: "Delete a saved version",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			it, err := findItemByName(cfg, args[0])
			if err != nil {
				return err
			}
			if err := source.DeleteVersion(it.Path, args[1]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted version %q of %q\n", args[1], args[0])
			return nil
		},
	}

	c.AddCommand(list, save, switchCmd, deleteCmd)
	return c
}

func newBackupCmd(load func() (config.Config, error)) *cobra.Command {
	c := &cobra.Command{
		Use:   "backup",
		Short: "Manage snapshots of target directories",
	}

	create := &cobra.Command{
		Use:   "create",
		Short: "Snapshot all target directories now",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			if len(cfg.Targets) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no targets configured; nothing to back up")
				return nil
			}
			anyExists := false
			for _, t := range cfg.Targets {
				if _, serr := os.Stat(t.Path); serr == nil {
					anyExists = true
					break
				}
			}
			if !anyExists {
				fmt.Fprintln(cmd.OutOrStdout(), "no existing target directories to back up")
				return nil
			}
			root, err := backup.DefaultRoot()
			if err != nil {
				return err
			}
			dir, err := backup.Create(cfg, root)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), dir)
			return nil
		},
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "List available snapshots",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := backup.DefaultRoot()
			if err != nil {
				return err
			}
			snaps, err := backup.List(root)
			if err != nil {
				return err
			}
			if len(snaps) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no backups found")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "TIMESTAMP\tTARGETS")
			for _, s := range snaps {
				names := make([]string, len(s.Targets))
				for i, t := range s.Targets {
					names[i] = t.Name
				}
				fmt.Fprintf(tw, "%s\t%s\n", s.Timestamp.Format("2006-01-02 15:04:05 UTC"),
					strings.Join(names, ", "))
			}
			return tw.Flush()
		},
	}

	var restoreLatest bool
	var restoreIndex int
	restore := &cobra.Command{
		Use:   "restore",
		Short: "Restore a snapshot back to original target paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			root, err := backup.DefaultRoot()
			if err != nil {
				return err
			}
			snaps, err := backup.List(root)
			if err != nil {
				return err
			}
			if len(snaps) == 0 {
				return fmt.Errorf("no backups found in %s", root)
			}

			var idx int
			switch {
			case restoreLatest:
				idx = 1
			case restoreIndex > 0:
				if restoreIndex > len(snaps) {
					return fmt.Errorf("index %d out of range (1-%d)", restoreIndex, len(snaps))
				}
				idx = restoreIndex
			default:
				fmt.Fprintln(cmd.OutOrStdout(), "Available snapshots:")
				for i, s := range snaps {
					names := make([]string, len(s.Targets))
					for j, t := range s.Targets {
						names[j] = t.Name
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  [%d] %s  (%s)\n",
						i+1, s.Timestamp.Format("2006-01-02 15:04:05 UTC"), strings.Join(names, ", "))
				}
				fmt.Fprint(cmd.OutOrStdout(), "Select snapshot number: ")
				if _, err := fmt.Fscan(cmd.InOrStdin(), &idx); err != nil || idx < 1 || idx > len(snaps) {
					return fmt.Errorf("invalid selection")
				}
			}
			chosen := snaps[idx-1]

			entries, err := os.ReadDir(root)
			if err != nil {
				return err
			}
			var snapshotDir string
			target := chosen.Timestamp.UTC().Format("20060102-150405")
			for _, e := range entries {
				if e.Name() == target {
					snapshotDir = filepath.Join(root, e.Name())
					break
				}
			}
			if snapshotDir == "" {
				return fmt.Errorf("snapshot directory not found")
			}

			if err := backup.Restore(snapshotDir, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "restored from %s\n", snapshotDir)
			return nil
		},
	}
	restore.Flags().BoolVar(&restoreLatest, "latest", false, "restore from the most recent snapshot without prompting")
	restore.Flags().IntVar(&restoreIndex, "index", 0, "restore snapshot by 1-based index (as shown by 'backup list')")

	var pruneKeep int
	prune := &cobra.Command{
		Use:   "prune",
		Short: "Delete old snapshots, keeping the most recent N",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := backup.DefaultRoot()
			if err != nil {
				return err
			}
			before, err := backup.List(root)
			if err != nil {
				return err
			}
			if err := backup.Prune(root, pruneKeep); err != nil {
				return err
			}
			after, err := backup.List(root)
			if err != nil {
				return err
			}
			deleted := len(before) - len(after)
			if deleted == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "nothing to prune (%d snapshots)\n", len(after))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "pruned %d snapshot(s), %d remaining\n", deleted, len(after))
			}
			return nil
		},
	}
	prune.Flags().IntVar(&pruneKeep, "keep", 5, "number of most recent snapshots to keep")

	c.AddCommand(create, list, restore, prune)
	// default subcommand: create
	c.RunE = create.RunE
	return c
}
