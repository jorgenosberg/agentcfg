package claudecfg

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/jorgenosberg/agentcfg/internal/fsutil"
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
func SetPluginEnabled(path, fullName string, enabled bool) error {
	return fsutil.EditJSON(path, []byte("{}"), func(doc map[string]json.RawMessage) error {
		enabledPlugins := map[string]bool{}
		if existing, ok := doc["enabledPlugins"]; ok {
			if err := json.Unmarshal(existing, &enabledPlugins); err != nil {
				return fmt.Errorf("parse enabledPlugins: %w", err)
			}
		}
		enabledPlugins[fullName] = enabled
		raw, err := json.Marshal(enabledPlugins)
		if err != nil {
			return err
		}
		doc["enabledPlugins"] = json.RawMessage(raw)
		return nil
	})
}
