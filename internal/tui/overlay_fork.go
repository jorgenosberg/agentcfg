package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jorgenosberg/agentcfg/internal/fork"
	"github.com/jorgenosberg/agentcfg/internal/plugins"
)

// ── forkOverlay ───────────────────────────────────────────────────────────────

// forkOverlay confirms forking an entire plugin bundle into the agentcfg fork
// marketplace before executing the operation.
type forkOverlay struct {
	plugin               plugins.Plugin
	forksRoot            string
	forksPath            string
	settingsPath         string
	knownMPPath          string
	installedPluginsPath string
}

func newForkOverlay(plugin plugins.Plugin, forksRoot, forksPath, settingsPath, knownMPPath, installedPluginsPath string) *forkOverlay {
	return &forkOverlay{
		plugin:               plugin,
		forksRoot:            forksRoot,
		forksPath:            forksPath,
		settingsPath:         settingsPath,
		knownMPPath:          knownMPPath,
		installedPluginsPath: installedPluginsPath,
	}
}

func (o *forkOverlay) Update(msg tea.Msg) (overlayModel, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return o, nil
	}
	switch key.String() {
	case "ctrl+c":
		return nil, tea.Quit
	case "esc":
		return nil, nil
	case "enter", "y", "Y":
		req := fork.Request{
			Plugin:                o.plugin,
			ForksRoot:             o.forksRoot,
			ForksPath:             o.forksPath,
			SettingsPath:          o.settingsPath,
			KnownMarketplacesPath: o.knownMPPath,
			InstalledPluginsPath:  o.installedPluginsPath,
		}
		return nil, func() tea.Msg {
			res, err := fork.Execute(req)
			if err != nil {
				return cfgReloadMsg{err: err}
			}
			return cfgReloadMsg{status: fmt.Sprintf("forked → %s", res.ForkFullName)}
		}
	}
	return o, nil
}

func (o *forkOverlay) View(w int) string {
	var sb strings.Builder
	title := fmt.Sprintf("Fork %q", o.plugin.FullName)

	sb.WriteString("Copies the full plugin bundle into:\n")
	sb.WriteString(dimStyle.Render("  ~/.agentcfg/forks/plugins/"+o.plugin.Name) + "\n\n")
	if len(o.plugin.Skills) > 0 {
		fmt.Fprintf(&sb, "Skills:  %s\n", strings.Join(o.plugin.Skills, ", "))
	}
	if len(o.plugin.Hooks) > 0 {
		fmt.Fprintf(&sb, "Hooks:   %s\n", strings.Join(o.plugin.Hooks, ", "))
	}
	if len(o.plugin.MCPServers) > 0 {
		fmt.Fprintf(&sb, "MCP:     %s\n", strings.Join(o.plugin.MCPServers, ", "))
	}
	sb.WriteString("\nUpstream plugin disabled, fork enabled.\n")
	fmt.Fprintf(&sb, "\n%s  %s", tabActiveStyle.Render("[ Enter ]"), dimStyle.Render("confirm · Esc cancel"))

	return renderOverlayBox(w, strings.TrimRight(sb.String(), "\n"), title, 60)
}
