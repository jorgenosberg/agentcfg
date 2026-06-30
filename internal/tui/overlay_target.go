package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jorgenosberg/agentcfg/internal/agent"
	"github.com/jorgenosberg/agentcfg/internal/catalog"
	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/icons"
)

// ── addTargetOverlay ──────────────────────────────────────────────────────────

type addTargetOverlay struct {
	cfgPath   string
	cfg       config.Config
	name      textinput.Model
	path      textinput.Model
	alias     textinput.Model
	agentType string // selected agent type; "" = none
	focused   int    // 0=name, 1=path, 2=agentType, 3=alias
	errMsg    string
}

func prevAgent(current string) string {
	names := append([]string{""}, agent.Names()...)
	for i, n := range names {
		if n == current {
			if i == 0 {
				return names[len(names)-1]
			}
			return names[i-1]
		}
	}
	return ""
}

func nextAgent(current string) string {
	names := append([]string{""}, agent.Names()...)
	for i, n := range names {
		if n == current {
			return names[(i+1)%len(names)]
		}
	}
	return names[0]
}

func newAddTargetOverlay(cfgPath string, cfg config.Config) (*addTargetOverlay, tea.Cmd) {
	name := textinput.New()
	name.Placeholder = `e.g. "claude-work"`
	name.Prompt = "Name:  "
	name.PromptStyle = dimStyle
	name.Width = 38
	path := textinput.New()
	path.Placeholder = "absolute path to agent config dir"
	path.Prompt = "Path:  "
	path.PromptStyle = dimStyle
	path.Width = 38
	alias := textinput.New()
	alias.Placeholder = `e.g. "claude" to group with other Claude targets`
	alias.Prompt = "Alias: "
	alias.PromptStyle = dimStyle
	alias.Width = 38
	cmd := name.Focus()
	return &addTargetOverlay{cfgPath: cfgPath, cfg: cfg, name: name, path: path, alias: alias}, cmd
}

func (o *addTargetOverlay) Update(msg tea.Msg) (overlayModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return nil, tea.Quit
		case "esc":
			return nil, nil
		case "tab", "down":
			o.blurCurrent()
			o.focused = (o.focused + 1) % 4
			return o, o.focusCurrent()
		case "shift+tab", "up":
			o.blurCurrent()
			o.focused = (o.focused - 1 + 4) % 4
			return o, o.focusCurrent()
		case "left":
			if o.focused == 2 {
				o.agentType = prevAgent(o.agentType)
				return o, nil
			}
		case "right":
			if o.focused == 2 {
				o.agentType = nextAgent(o.agentType)
				return o, nil
			}
		case "enter":
			if o.focused < 3 {
				o.blurCurrent()
				o.focused++
				return o, o.focusCurrent()
			}
			// focused == 3: submit
			name := strings.TrimSpace(o.name.Value())
			rawPath := strings.TrimSpace(o.path.Value())
			if name == "" || rawPath == "" {
				o.errMsg = "name and path are required"
				return o, nil
			}
			for _, t := range o.cfg.Targets {
				if t.Name == name {
					o.errMsg = fmt.Sprintf("target %q already exists", name)
					return o, nil
				}
			}
			abs, err := filepath.Abs(rawPath)
			if err != nil {
				abs = rawPath
			}
			cfg := o.cfg
			t := catalog.TargetFor(o.agentType, abs, name)
			if a := strings.TrimSpace(o.alias.Value()); a != "" {
				t.Alias = a
			}
			cfg.Targets = append(cfg.Targets, t)
			cfgPath := o.cfgPath
			return nil, func() tea.Msg { return cfgReloadMsg{err: config.Save(cfgPath, cfg)} }
		}
		var cmd tea.Cmd
		switch o.focused {
		case 0:
			o.name, cmd = o.name.Update(msg)
		case 1:
			o.path, cmd = o.path.Update(msg)
		case 3:
			o.alias, cmd = o.alias.Update(msg)
		}
		return o, cmd
	default:
		var cmd tea.Cmd
		switch o.focused {
		case 0:
			o.name, cmd = o.name.Update(msg)
		case 1:
			o.path, cmd = o.path.Update(msg)
		case 3:
			o.alias, cmd = o.alias.Update(msg)
		}
		return o, cmd
	}
}

func (o *addTargetOverlay) blurCurrent() {
	switch o.focused {
	case 0:
		o.name.Blur()
	case 1:
		o.path.Blur()
	case 3:
		o.alias.Blur()
	}
}

func (o *addTargetOverlay) focusCurrent() tea.Cmd {
	switch o.focused {
	case 0:
		return o.name.Focus()
	case 1:
		return o.path.Focus()
	case 3:
		return o.alias.Focus()
	}
	return nil
}

func (o *addTargetOverlay) View(w int) string {
	var sb strings.Builder

	// Name field
	if o.focused == 0 {
		sb.WriteString(cursorStyle.Render("▶ "))
	} else {
		sb.WriteString("  ")
	}
	sb.WriteString(o.name.View())
	sb.WriteString("\n\n")

	// Path field
	if o.focused == 1 {
		sb.WriteString(cursorStyle.Render("▶ "))
	} else {
		sb.WriteString("  ")
	}
	sb.WriteString(o.path.View())
	sb.WriteString("\n\n")

	// Agent type field
	if o.focused == 2 {
		sb.WriteString(cursorStyle.Render("▶ "))
	} else {
		sb.WriteString("  ")
	}
	sb.WriteString(dimStyle.Render("Agent: "))
	if o.agentType == "" {
		sb.WriteString(dimStyle.Render("(none)"))
	} else {
		sb.WriteString(o.agentType)
	}
	if o.focused == 2 {
		sb.WriteString(dimStyle.Render(" ◀ ▶"))
	}
	sb.WriteString("\n\n")

	// Alias field (optional)
	if o.focused == 3 {
		sb.WriteString(cursorStyle.Render("▶ "))
	} else {
		sb.WriteString("  ")
	}
	sb.WriteString(o.alias.View())
	sb.WriteString("\n\n")

	if o.errMsg != "" {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render("  "+o.errMsg) + "\n\n")
	}
	sb.WriteString(dimStyle.Render("tab next field · ←/→ cycle agent · enter/tab confirm · esc cancel"))
	return renderOverlayBox(w, sb.String(), "Add target", 62)
}

// ── addProjectOverlay ─────────────────────────────────────────────────────────

type addProjectOverlay struct {
	cfgPath string
	cfg     config.Config
	name    textinput.Model
	path    textinput.Model
	focused int
	errMsg  string
}

func newAddProjectOverlay(cfgPath string, cfg config.Config) (*addProjectOverlay, tea.Cmd) {
	name := textinput.New()
	name.Placeholder = `e.g. "myapp"`
	name.Prompt = "Name: "
	name.PromptStyle = dimStyle
	name.Width = 38
	path := textinput.New()
	path.Placeholder = "path to repository root"
	path.Prompt = "Path: "
	path.PromptStyle = dimStyle
	path.Width = 38
	cmd := name.Focus()
	return &addProjectOverlay{cfgPath: cfgPath, cfg: cfg, name: name, path: path}, cmd
}

func (o *addProjectOverlay) inputs() []*textinput.Model {
	return []*textinput.Model{&o.name, &o.path}
}

func (o *addProjectOverlay) Update(msg tea.Msg) (overlayModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return nil, tea.Quit
		case "esc":
			return nil, nil
		case "tab", "down":
			inp := o.inputs()
			inp[o.focused].Blur()
			o.focused = (o.focused + 1) % len(inp)
			return o, inp[o.focused].Focus()
		case "shift+tab", "up":
			inp := o.inputs()
			inp[o.focused].Blur()
			o.focused = (o.focused - 1 + len(inp)) % len(inp)
			return o, inp[o.focused].Focus()
		case "enter":
			if o.focused == 0 {
				o.name.Blur()
				o.focused = 1
				return o, o.path.Focus()
			}
			name, rawPath := strings.TrimSpace(o.name.Value()), strings.TrimSpace(o.path.Value())
			if name == "" || rawPath == "" {
				o.errMsg = "both fields required"
				return o, nil
			}
			for _, p := range o.cfg.Projects {
				if p.Name == name {
					o.errMsg = fmt.Sprintf("project %q already exists", name)
					return o, nil
				}
			}
			abs, err := filepath.Abs(rawPath)
			if err != nil {
				abs = rawPath
			}
			cfg := o.cfg
			cfg.Projects = append(cfg.Projects, config.Project{Name: name, Path: abs})
			cfgPath := o.cfgPath
			return nil, func() tea.Msg { return cfgReloadMsg{err: config.Save(cfgPath, cfg)} }
		}
		var cmd tea.Cmd
		if o.focused == 0 {
			o.name, cmd = o.name.Update(msg)
		} else {
			o.path, cmd = o.path.Update(msg)
		}
		return o, cmd
	default:
		var cmd tea.Cmd
		if o.focused == 0 {
			o.name, cmd = o.name.Update(msg)
		} else {
			o.path, cmd = o.path.Update(msg)
		}
		return o, cmd
	}
}

func (o *addProjectOverlay) View(w int) string {
	var sb strings.Builder
	for i, inp := range []textinput.Model{o.name, o.path} {
		if i == o.focused {
			sb.WriteString(cursorStyle.Render("▶ "))
		} else {
			sb.WriteString("  ")
		}
		sb.WriteString(inp.View())
		sb.WriteString("\n\n")
	}
	if o.errMsg != "" {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render("  "+o.errMsg) + "\n\n")
	}
	sb.WriteString(dimStyle.Render("tab next field · enter submit · esc cancel"))
	return renderOverlayBox(w, sb.String(), "Add project", 54)
}

// ── discoverOverlay ───────────────────────────────────────────────────────────

type discoverOverlay struct {
	cfgPath    string
	cfg        config.Config
	candidates []config.Target
	ms         multiSelect
	empty      bool
}

func newDiscoverOverlay(cfgPath string, cfg config.Config) *discoverOverlay {
	found := catalog.Discover()
	existing := make(map[string]bool, len(cfg.Targets))
	for _, t := range cfg.Targets {
		existing[t.Name] = true
	}
	var candidates []config.Target
	for _, t := range found {
		if !existing[t.Name] {
			candidates = append(candidates, t)
		}
	}
	if len(candidates) == 0 {
		return &discoverOverlay{cfgPath: cfgPath, cfg: cfg, empty: true}
	}
	items := make([]msItem, len(candidates))
	for i, t := range candidates {
		badge := icons.TextBadge(t.Name, 3)
		items[i] = msItem{
			label: fmt.Sprintf("%s  %-10s  %s", badge, t.Name, t.Path),
			value: t.Name,
		}
	}
	return &discoverOverlay{
		cfgPath:    cfgPath,
		cfg:        cfg,
		candidates: candidates,
		ms:         newMultiSelect(items, 12),
	}
}

func (o *discoverOverlay) Update(msg tea.Msg) (overlayModel, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return o, nil
	}
	switch key.String() {
	case "ctrl+c":
		return nil, tea.Quit
	case "esc", "q":
		return nil, nil
	case "enter":
		if o.empty {
			return nil, nil
		}
		cfg := o.cfg
		for _, it := range o.ms.selectedItems() {
			for _, t := range o.candidates {
				if t.Name == it.value {
					cfg.Targets = append(cfg.Targets, t)
					break
				}
			}
		}
		cfgPath := o.cfgPath
		return nil, func() tea.Msg { return cfgReloadMsg{err: config.Save(cfgPath, cfg)} }
	}
	if !o.empty {
		o.ms.handleKey(key.String())
	}
	return o, nil
}

func (o *discoverOverlay) View(w int) string {
	if o.empty {
		content := dimStyle.Render("No new agent directories found on disk.") +
			"\n\n" + dimStyle.Render("esc close")
		return renderOverlayBox(w, content, "Discover agents", 52)
	}
	selCount := len(o.ms.selectedItems())
	header := dimStyle.Render(fmt.Sprintf("%d / %d selected", selCount, len(o.ms.items)))
	content := header + "\n\n" + o.ms.View() + "\n\n" +
		dimStyle.Render("j/k move · space toggle · ctrl+a all/none · enter confirm · esc cancel")
	return renderOverlayBox(w, content, "Discover agents", max(52, w*3/5))
}
