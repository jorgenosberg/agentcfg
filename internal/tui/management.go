package tui

import "github.com/jorgenosberg/agentcfg/internal/config"

func targetNamesFromConfig(cfg config.Config) []string {
	names := make([]string, len(cfg.Targets))
	for i, t := range cfg.Targets {
		names[i] = t.Name
	}
	return names
}
