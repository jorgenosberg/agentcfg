// Package fsutil provides shared file-system helpers used across packages that
// must not import each other.
package fsutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AtomicWrite writes data to path via a sibling temp file + rename so a crash
// mid-write never leaves a partial file. The parent directory is created if it
// does not exist.
func AtomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".agentcfg-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op after successful rename

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// EditJSON reads path as a JSON object, applies edit to a map of its top-level
// keys, then writes the result back atomically. If path does not exist,
// defaultDoc is used as the starting document (must be valid JSON). Fields not
// touched by edit are preserved verbatim.
func EditJSON(path string, defaultDoc []byte, edit func(map[string]json.RawMessage) error) error {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		raw = defaultDoc
	} else if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	if doc == nil {
		doc = map[string]json.RawMessage{}
	}

	if err := edit(doc); err != nil {
		return err
	}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return AtomicWrite(path, out)
}
