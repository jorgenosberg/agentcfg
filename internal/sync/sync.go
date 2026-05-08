package sync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/source"
)

// Status describes the state of one Item against one Target.
type Status string

const (
	StatusLinked  Status = "linked"  // installed as symlink pointing at source
	StatusCopied  Status = "copied"  // installed as a copy (snapshot)
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
	if strategy == config.StrategyCopy && sameContent(dest, src) {
		return StatusCopied
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
	return filepath.Walk(src, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
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
