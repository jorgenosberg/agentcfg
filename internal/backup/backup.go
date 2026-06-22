package backup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/sync"
)

// TargetSnapshot records one target's backup location.
type TargetSnapshot struct {
	Name string `json:"name"`
	Path string `json:"path"` // original path, e.g. ~/.claude
	Dir  string `json:"dir"`  // relative path under the snapshot root
}

// Snapshot describes a single backup.
type Snapshot struct {
	Timestamp time.Time        `json:"timestamp"`
	Targets   []TargetSnapshot `json:"targets"`
}

// Create copies every configured target directory into a timestamped subdirectory
// under root. Returns the snapshot directory path.
func Create(cfg config.Config, root string) (string, error) {
	ts := time.Now().UTC()
	dir := filepath.Join(root, ts.Format("20060102-150405"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	snap := Snapshot{Timestamp: ts}
	for _, t := range cfg.Targets {
		if _, err := os.Stat(t.Path); err != nil {
			continue // target doesn't exist yet; skip
		}
		rel := t.Name
		dest := filepath.Join(dir, rel)
		if err := sync.CopyAny(t.Path, dest); err != nil {
			return "", fmt.Errorf("backup target %s: %w", t.Name, err)
		}
		snap.Targets = append(snap.Targets, TargetSnapshot{
			Name: t.Name,
			Path: t.Path,
			Dir:  rel,
		})
	}

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o644); err != nil {
		return "", err
	}
	return dir, nil
}

// List reads all snapshots under root. Returns them sorted newest first.
func List(root string) ([]Snapshot, error) {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var snaps []Snapshot
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		manifest := filepath.Join(root, e.Name(), "manifest.json")
		data, err := os.ReadFile(manifest)
		if err != nil {
			continue // skip corrupted/incomplete backups
		}
		var s Snapshot
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		snaps = append(snaps, s)
	}
	sort.Slice(snaps, func(i, j int) bool {
		return snaps[i].Timestamp.After(snaps[j].Timestamp)
	})
	return snaps, nil
}

// Restore copies files from snapshotDir back to the original target paths.
// Existing content in target paths is overwritten.
func Restore(snapshotDir string, cfg config.Config) error {
	data, err := os.ReadFile(filepath.Join(snapshotDir, "manifest.json"))
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	for _, ts := range snap.Targets {
		src := filepath.Join(snapshotDir, ts.Dir)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := os.RemoveAll(ts.Path); err != nil {
			return fmt.Errorf("clear %s: %w", ts.Path, err)
		}
		if err := sync.CopyAny(src, ts.Path); err != nil {
			return fmt.Errorf("restore %s: %w", ts.Name, err)
		}
	}
	return nil
}

// Prune removes old snapshots, keeping the `keep` most recent ones.
// It is a no-op when the number of snapshots is already ≤ keep.
func Prune(root string, keep int) error {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	snaps, err := List(root)
	if err != nil {
		return err
	}
	if len(snaps) <= keep {
		return nil
	}
	retained := make(map[string]bool, keep)
	for _, s := range snaps[:keep] {
		retained[s.Timestamp.UTC().Format("20060102-150405")] = true
	}
	for _, e := range entries {
		if e.IsDir() && !retained[e.Name()] {
			_ = os.RemoveAll(filepath.Join(root, e.Name()))
		}
	}
	return nil
}

// DefaultRoot returns ~/.agentcfg/backups.
func DefaultRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agentcfg", "backups"), nil
}
