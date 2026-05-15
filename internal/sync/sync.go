package sync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/lock"
	"github.com/jorgenosberg/agentcfg/internal/source"
)

// Status describes the state of one Item against one Target.
type Status string

const (
	StatusLinked  Status = "linked"  // installed as symlink pointing at source
	StatusCopied  Status = "copied"  // installed as a copy (snapshot)
	StatusDrifted Status = "drifted" // copy exists but source has changed
	StatusForeign Status = "foreign" // present but not managed by agentcfg
	StatusAbsent  Status = "absent"  // not installed
)

// Entry is the per-(target, item) state used for listing.
type Entry struct {
	Target config.Target
	Item   source.Item
	Status Status
	Dest   string // resolved install path on target side
}

// Inspect computes Entries for all items across all targets.
func Inspect(cfg config.Config, items []source.Item) []Entry {
	var out []Entry
	for _, t := range cfg.Targets {
		strategy := t.ResolveStrategy(cfg.DefaultStrategy)
		for _, it := range items {
			dest := destPath(t, it)
			out = append(out, Entry{
				Target: t,
				Item:   it,
				Status: statusOf(dest, it.Path, strategy),
				Dest:   dest,
			})
		}
	}
	return out
}

// Install creates the link or copy from item to target.
// strategy is the resolved strategy ("link" or "copy"); callers pass
// target.ResolveStrategy(cfg.DefaultStrategy).
func Install(t config.Target, strategy string, it source.Item) (Status, error) {
	dest := destPath(t, it)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}

	switch s := statusOf(dest, it.Path, strategy); s {
	case StatusLinked, StatusCopied:
		return s, nil
	case StatusForeign:
		return s, fmt.Errorf("%s exists and is not managed by agentcfg", dest)
	case StatusDrifted:
		// remove stale copy before reinstalling
		if err := os.RemoveAll(dest); err != nil {
			return "", err
		}
	}

	switch strategy {
	case config.StrategyLink:
		if err := os.Symlink(it.Path, dest); err != nil {
			return "", err
		}
		return StatusLinked, nil
	case config.StrategyCopy:
		if err := copyAny(it.Path, dest); err != nil {
			return "", err
		}
		return StatusCopied, nil
	default:
		return "", fmt.Errorf("unknown strategy %q", strategy)
	}
}

// Uninstall removes the managed entry. It refuses to delete foreign content.
func Uninstall(t config.Target, strategy string, it source.Item) error {
	dest := destPath(t, it)
	switch statusOf(dest, it.Path, strategy) {
	case StatusAbsent:
		return nil
	case StatusForeign:
		return fmt.Errorf("%s not managed by agentcfg; refusing to remove", dest)
	}
	return os.RemoveAll(dest)
}

func destPath(t config.Target, it source.Item) string {
	sub := t.SubdirFor(it.Kind)
	if sub == "" {
		return filepath.Join(t.Path, it.Name)
	}
	return filepath.Join(t.Path, sub, it.Name)
}

func statusOf(dest, src, strategy string) Status {
	fi, err := os.Lstat(dest)
	if os.IsNotExist(err) {
		return StatusAbsent
	}
	if err != nil {
		return StatusForeign
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(dest)
		if err != nil {
			return StatusForeign
		}
		if target == src {
			return StatusLinked
		}
		return StatusForeign
	}
	if strategy == config.StrategyCopy {
		if sameContent(dest, src) {
			return StatusCopied
		}
		return StatusDrifted
	}
	return StatusForeign
}

// sameContent is a coarse check: matching size and (for files) byte equality.
// Directories are compared by recursive walk equality on first divergence.
func sameContent(a, b string) bool {
	fa, errA := os.Stat(a)
	fb, errB := os.Stat(b)
	if errA != nil || errB != nil {
		return false
	}
	if fa.IsDir() != fb.IsDir() {
		return false
	}
	if !fa.IsDir() {
		if fa.Size() != fb.Size() {
			return false
		}
		ba, errA := os.ReadFile(a)
		bb, errB := os.ReadFile(b)
		if errA != nil || errB != nil {
			return false
		}
		return string(ba) == string(bb)
	}
	// directory: shallow check on entry names
	ea, errA := os.ReadDir(a)
	eb, errB := os.ReadDir(b)
	if errA != nil || errB != nil || len(ea) != len(eb) {
		return false
	}
	for i := range ea {
		if ea[i].Name() != eb[i].Name() {
			return false
		}
	}
	return true
}

// SyncResult holds the outcome for one (target, item) pair after a Sync run.
type SyncResult struct {
	Entry     Entry
	OldStatus Status
	Err       error
}

// Sync installs all absent or drifted items across all targets. When dryRun is
// true it returns what would happen without making any changes. Successful
// installs are recorded in lck (caller is responsible for persisting it).
func Sync(cfg config.Config, items []source.Item, lck lock.Lock, dryRun bool) []SyncResult {
	entries := Inspect(cfg, items)
	var out []SyncResult
	for _, e := range entries {
		if e.Status != StatusAbsent && e.Status != StatusDrifted {
			continue
		}
		if dryRun {
			out = append(out, SyncResult{Entry: e, OldStatus: e.Status})
			continue
		}
		strategy := e.Target.ResolveStrategy(cfg.DefaultStrategy)
		newStatus, err := Install(e.Target, strategy, e.Item)
		if err != nil {
			out = append(out, SyncResult{Entry: e, OldStatus: e.Status, Err: err})
			continue
		}
		if lck != nil {
			if h, hashErr := lock.HashPath(e.Item.Path); hashErr == nil {
				lck[e.Dest] = lock.Entry{
					Hash:        h,
					InstalledAt: time.Now().UTC(),
				}
			}
		}
		oldStatus := e.Status
		e.Status = newStatus
		out = append(out, SyncResult{Entry: e, OldStatus: oldStatus})
	}
	return out
}

// CopyAny copies a file or directory tree from src to dst. Parent dirs of
// dst must already exist.
func CopyAny(src, dst string) error { return copyAny(src, dst) }

func copyAny(src, dst string) error {
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst, fi.Mode())
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	// Resolve symlinked root so Walk descends into the target directory.
	root, err := filepath.EvalSymlinks(src)
	if err != nil {
		return err
	}
	return filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, fi.Mode())
		}
		return copyFile(p, target, fi.Mode())
	})
}
