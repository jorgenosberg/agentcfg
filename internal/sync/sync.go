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
	StatusLinked        Status = "linked"        // installed as symlink pointing at source
	StatusCopied        Status = "copied"        // installed as a copy (snapshot)
	StatusDrifted       Status = "drifted"       // copy exists but source has changed
	StatusUnmanaged     Status = "unmanaged"     // present but not managed by agentcfg
	StatusAbsent        Status = "absent"        // not installed
	StatusNotApplicable Status = "n/a"           // target does not support this item kind
	StatusDisabled      Status = "disabled"      // user has disabled this item for this target
	StatusPluginOwned   Status = "plugin-owned"  // from an enabled plugin, not in agentcfg source
	StatusPluginSibling Status = "plugin-sibling" // from a disabled plugin, not yet forked
)

// PluginRef identifies the Claude Code plugin that produced a ghost entry.
type PluginRef struct {
	FullName string // "plugin@marketplace"
}

// Entry is the per-(target, item) state used for listing.
type Entry struct {
	Target config.Target
	Item   source.Item
	Status Status
	Dest   string     // resolved install path on target side
	Plugin *PluginRef // non-nil for plugin-derived ghost entries only
}

// Inspect computes Entries for all items across all targets.
func Inspect(cfg config.Config, items []source.Item) []Entry {
	var out []Entry
	for _, t := range cfg.Targets {
		strategy := t.ResolveStrategy(cfg.DefaultStrategy)
		for _, it := range items {
			if t.Excludes(it) {
				continue
			}
			if t.IsDisabled(it) {
				out = append(out, Entry{
					Target: t,
					Item:   it,
					Status: StatusDisabled,
					Dest:   destPath(t, it),
				})
				continue
			}
			if !t.SupportsKind(it.Kind) {
				out = append(out, Entry{
					Target: t,
					Item:   it,
					Status: StatusNotApplicable,
					Dest:   "",
				})
				continue
			}
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

// ScanTargetDirs scans each target's install directories and returns Entries
// for items actually present there. Items not matching any source item are
// returned with StatusUnmanaged.
func ScanTargetDirs(cfg config.Config, sourceItems []source.Item) []Entry {
	srcMap := make(map[string]source.Item, len(sourceItems))
	for _, it := range sourceItems {
		srcMap[it.Kind+"/"+it.Name] = it
	}
	var out []Entry
	for _, t := range cfg.Targets {
		strategy := t.ResolveStrategy(cfg.DefaultStrategy)
		items, err := source.ScanWith(t.Path, t.SupportedSubdirs())
		if err != nil {
			continue
		}
		for _, it := range items {
			key := it.Kind + "/" + it.Name
			if srcItem, ok := srcMap[key]; ok {
				out = append(out, Entry{
					Target: t,
					Item:   it,
					Status: statusOf(it.Path, srcItem.Path, strategy),
					Dest:   it.Path,
				})
			} else {
				out = append(out, Entry{
					Target: t,
					Item:   it,
					Status: StatusUnmanaged,
					Dest:   it.Path,
				})
			}
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
	case StatusUnmanaged:
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
		if err := CopyAny(it.Path, dest); err != nil {
			return "", err
		}
		return StatusCopied, nil
	default:
		return "", fmt.Errorf("unknown strategy %q", strategy)
	}
}

// Uninstall removes the managed entry. It refuses to delete unmanaged content.
func Uninstall(t config.Target, strategy string, it source.Item) error {
	dest := destPath(t, it)
	switch statusOf(dest, it.Path, strategy) {
	case StatusAbsent:
		return nil
	case StatusUnmanaged:
		return fmt.Errorf("%s not managed by agentcfg; refusing to remove", dest)
	}
	return os.RemoveAll(dest)
}

// Adopt replaces an unmanaged file at the target with a managed install.
// It is equivalent to Install for absent, drifted, linked, or copied entries.
func Adopt(t config.Target, strategy string, it source.Item) (Status, error) {
	dest := destPath(t, it)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	if statusOf(dest, it.Path, strategy) == StatusUnmanaged {
		if err := os.RemoveAll(dest); err != nil {
			return "", fmt.Errorf("remove unmanaged %s: %w", dest, err)
		}
	}
	return Install(t, strategy, it)
}

// Toggle adds (disable=true) or removes (disable=false) item.Name from the
// named target's Disabled list, then installs or uninstalls accordingly.
// It reloads the config from cfgPath before modifying to avoid stale-write
// races when called in a loop over multiple targets.
func Toggle(cfgPath, targetName string, item source.Item, disable bool) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	idx := -1
	for i, t := range cfg.Targets {
		if t.Name == targetName {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("target %q not found in config", targetName)
	}
	t := &cfg.Targets[idx]
	strategy := t.ResolveStrategy(cfg.DefaultStrategy)

	if disable {
		if t.IsDisabled(item) {
			return nil // already disabled — idempotent
		}
		t.Disabled = append(t.Disabled, item.Name)
		dest := destPath(*t, item)
		st := statusOf(dest, item.Path, strategy)
		// Skip Uninstall when the file is already absent or already unmanaged
		// (e.g. after Unmanage was called). The Disabled flag is what matters here.
		if st != StatusAbsent && st != StatusUnmanaged {
			if err := Uninstall(*t, strategy, item); err != nil {
				return fmt.Errorf("uninstall %s from %s: %w", item.Name, t.Name, err)
			}
		}
	} else {
		kept := make([]string, 0, len(t.Disabled))
		for _, d := range t.Disabled {
			if d != item.Name && d != item.Kind+"/"+item.Name {
				kept = append(kept, d)
			}
		}
		t.Disabled = kept
		if !t.Excludes(item) && t.SupportsKind(item.Kind) {
			if _, err := Install(*t, strategy, item); err != nil {
				return fmt.Errorf("install %s to %s: %w", item.Name, t.Name, err)
			}
		}
	}
	return config.Save(cfgPath, cfg)
}

// Unmanage copies the source file to dest as a real file, replacing any
// existing symlink or managed copy. If dest is already an unmanaged real
// file, it is left untouched. Callers should call Toggle with disable=true
// after Unmanage to prevent future syncs from reinstalling the item.
func Unmanage(t config.Target, strategy string, it source.Item) error {
	dest := destPath(t, it)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}
	st := statusOf(dest, it.Path, strategy)
	if st == StatusUnmanaged {
		return nil
	}
	if st != StatusAbsent {
		if err := os.RemoveAll(dest); err != nil {
			return fmt.Errorf("remove existing: %w", err)
		}
	}
	return CopyAny(it.Path, dest)
}

func destPath(t config.Target, it source.Item) string {
	sub := t.SubdirFor(it.Kind)
	name := t.DestNameFor(it.Kind, it.Name)
	if sub == "" {
		return filepath.Join(t.Path, name)
	}
	return filepath.Join(t.Path, sub, name)
}

func statusOf(dest, src, strategy string) Status {
	fi, err := os.Lstat(dest)
	if os.IsNotExist(err) {
		return StatusAbsent
	}
	if err != nil {
		return StatusUnmanaged
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(dest)
		if err != nil {
			return StatusUnmanaged
		}
		if target == src {
			return StatusLinked
		}
		return StatusUnmanaged
	}
	if strategy == config.StrategyCopy {
		if sameContent(dest, src) {
			return StatusCopied
		}
		return StatusDrifted
	}
	return StatusUnmanaged
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

// Sync installs all absent or drifted items across all targets. When force is
// true, unmanaged entries are also adopted (existing files replaced). When
// dryRun is true it returns what would happen without making any changes.
// Successful installs are recorded in lck (caller is responsible for persisting it).
func Sync(cfg config.Config, items []source.Item, lck lock.Lock, dryRun, force bool) []SyncResult {
	entries := Inspect(cfg, items)
	var out []SyncResult
	for _, e := range entries {
		if e.Status != StatusAbsent && e.Status != StatusDrifted && !(force && e.Status == StatusUnmanaged) {
			continue
		}
		if dryRun {
			out = append(out, SyncResult{Entry: e, OldStatus: e.Status})
			continue
		}
		strategy := e.Target.ResolveStrategy(cfg.DefaultStrategy)
		install := Install
		if force && e.Status == StatusUnmanaged {
			install = Adopt
		}
		newStatus, err := install(e.Target, strategy, e.Item)
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

// ImportItem copies a source item into sourceRoot under the subdirectory
// determined by its kind. If the destination already exists and force is false,
// it returns (true, nil). If force is true the existing destination is removed
// first. Use this instead of open-coding the destSub/destDir/CopyAny pattern.
func ImportItem(sourceRoot string, it source.Item, force bool) (skipped bool, err error) {
	destSub := source.DefaultSubdirs[it.Kind]
	destDir := filepath.Join(sourceRoot, destSub)
	dest := filepath.Join(destDir, it.Name)
	if _, statErr := os.Lstat(dest); statErr == nil {
		if !force {
			return true, nil
		}
		_ = os.RemoveAll(dest)
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return false, err
	}
	return false, CopyAny(it.Path, dest)
}

// CopyAny copies a file or directory tree from src to dst, following symlinks
// at the root. Parent dirs of dst must already exist.
func CopyAny(src, dst string) error {
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
