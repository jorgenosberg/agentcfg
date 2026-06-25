package claudecfg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jorgenosberg/agentcfg/internal/paths"
)

// DefaultPath returns the Claude Code settings file path.
func DefaultPath() (string, error) {
	home, err := paths.Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// SetPluginEnabled sets the enabled state of a plugin in ~/.claude/settings.json.
// All fields not related to enabledPlugins are preserved verbatim.
// The file is written atomically via a temp file + rename.
func SetPluginEnabled(path, fullName string, enabled bool) error {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		raw = []byte("{}")
	} else if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}

	// Parse into a generic map to preserve unknown fields.
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse settings: %w", err)
	}

	// Read existing enabledPlugins or start fresh.
	enabledPlugins := map[string]bool{}
	if existing, ok := doc["enabledPlugins"]; ok {
		if err := json.Unmarshal(existing, &enabledPlugins); err != nil {
			return fmt.Errorf("parse enabledPlugins: %w", err)
		}
	}

	enabledPlugins[fullName] = enabled

	pluginsRaw, err := json.Marshal(enabledPlugins)
	if err != nil {
		return err
	}
	doc["enabledPlugins"] = json.RawMessage(pluginsRaw)

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')

	return atomicWrite(path, out)
}

// atomicWrite writes data to path via a sibling temp file + rename,
// so a crash mid-write never leaves a partial file.
func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings-*.json.tmp")
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
