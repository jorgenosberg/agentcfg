package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jorgenosberg/agentcfg/internal/source"
)

// ── saveVersionOverlay ────────────────────────────────────────────────────────

type saveVersionOverlay struct {
	item   source.Item
	input  textinput.Model
	errMsg string
}

func newSaveVersionOverlay(item source.Item) (*saveVersionOverlay, tea.Cmd) {
	in := textinput.New()
	in.Placeholder = `e.g. "v1", "before-refactor"`
	in.Prompt = "Name: "
	in.PromptStyle = dimStyle
	in.Width = 38
	cmd := in.Focus()
	return &saveVersionOverlay{item: item, input: in}, cmd
}

func (o *saveVersionOverlay) Update(msg tea.Msg) (overlayModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return nil, nil
		case "enter":
			name := strings.TrimSpace(o.input.Value())
			if name == "" {
				o.errMsg = "version name is required"
				return o, nil
			}
			item := o.item
			if err := source.SaveVersion(item.Path, name); err != nil {
				o.errMsg = err.Error()
				return o, nil
			}
			return nil, func() tea.Msg {
				return cfgReloadMsg{status: fmt.Sprintf("saved version %q of %q", name, item.Name)}
			}
		}
	}
	var cmd tea.Cmd
	o.input, cmd = o.input.Update(msg)
	return o, cmd
}

func (o *saveVersionOverlay) View(w int) string {
	var sb strings.Builder
	sb.WriteString("  ")
	sb.WriteString(o.input.View())
	sb.WriteString("\n\n")
	if o.errMsg != "" {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render("  "+o.errMsg) + "\n\n")
	}
	sb.WriteString(dimStyle.Render("enter save · esc cancel"))
	return renderOverlayBox(w, sb.String(), fmt.Sprintf("Save version of %q", o.item.Name), 52)
}
