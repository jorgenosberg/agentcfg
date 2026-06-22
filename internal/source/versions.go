package source

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// VersionsDir returns the directory where saved versions of itemPath are stored.
// It lives at <parent>/.versions/<basename> and is automatically skipped by Scan.
func VersionsDir(itemPath string) string {
	return filepath.Join(filepath.Dir(itemPath), ".versions", filepath.Base(itemPath))
}

// ListVersions returns the names of all saved versions of itemPath, sorted alphabetically.
// Returns nil, nil when no versions have been saved yet.
func ListVersions(itemPath string) ([]string, error) {
	dir := VersionsDir(itemPath)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !hidden(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// SaveVersion copies the current active item to <versions-dir>/<name>.
// An existing version with the same name is overwritten.
func SaveVersion(itemPath, name string) error {
	if name == "" {
		return fmt.Errorf("version name must not be empty")
	}
	dir := VersionsDir(itemPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create versions dir: %w", err)
	}
	dest := filepath.Join(dir, name)
	_ = os.RemoveAll(dest)
	return vcopyAny(itemPath, dest)
}

// SwitchVersion replaces the active item with the named saved version.
// Before switching, the current active content is saved as "previous" so it
// can be recovered if needed. The switch itself is atomic at the file level.
func SwitchVersion(itemPath, name string) error {
	src := filepath.Join(VersionsDir(itemPath), name)
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("version %q not found", name)
	}
	// Auto-save current state before replacing it.
	if _, err := os.Stat(itemPath); err == nil {
		if err := SaveVersion(itemPath, "previous"); err != nil {
			return fmt.Errorf("save previous: %w", err)
		}
	}
	tmp := itemPath + ".agentcfg-swap"
	if err := vcopyAny(src, tmp); err != nil {
		return fmt.Errorf("prepare version: %w", err)
	}
	if err := os.RemoveAll(itemPath); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("remove active: %w", err)
	}
	return os.Rename(tmp, itemPath)
}

// DeleteVersion removes a saved version.
func DeleteVersion(itemPath, name string) error {
	path := filepath.Join(VersionsDir(itemPath), name)
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("version %q not found", name)
	}
	return os.RemoveAll(path)
}

func vcopyAny(src, dst string) error {
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return vcopyDir(src, dst)
	}
	return vcopyFile(src, dst, fi.Mode())
}

func vcopyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func vcopyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := vcopyDir(s, d); err != nil {
				return err
			}
		} else {
			fi, err := e.Info()
			if err != nil {
				return err
			}
			if err := vcopyFile(s, d, fi.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}
