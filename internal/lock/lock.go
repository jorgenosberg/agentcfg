package lock

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Entry records a single installed item's hash and install time.
type Entry struct {
	Hash        string    `json:"hash"`
	InstalledAt time.Time `json:"installed_at"`
}

// Lock maps absolute dest paths to their install records.
type Lock map[string]Entry

// Load reads the lockfile at path. Returns an empty Lock if the file is missing.
func Load(path string) (Lock, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Lock{}, nil
	}
	if err != nil {
		return nil, err
	}
	var l Lock
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, err
	}
	return l, nil
}

// Save writes the lock to path, creating parent directories as needed.
func Save(path string, l Lock) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// DefaultPath returns ~/.agentcfg/locks.json.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agentcfg", "locks.json"), nil
}

// HashPath returns a SHA-256 hex digest for a file or directory.
// For directories, the hash covers the sorted list of relative paths + file contents.
func HashPath(path string) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	if !fi.IsDir() {
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer f.Close()
		if _, err := io.Copy(h, f); err != nil {
			return "", err
		}
		return hex.EncodeToString(h.Sum(nil)), nil
	}

	var paths []string
	err = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, err := filepath.Rel(path, p)
			if err != nil {
				return err
			}
			paths = append(paths, rel)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	for _, rel := range paths {
		h.Write([]byte(rel + "\n"))
		f, err := os.Open(filepath.Join(path, rel))
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(h, f)
		f.Close()
		if copyErr != nil {
			return "", copyErr
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
