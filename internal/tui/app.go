package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/source"
	"github.com/jorgenosberg/agentcfg/internal/sync"
	"github.com/jorgenosberg/agentcfg/internal/version"
)

// Run starts the lazycfg TUI.
func Run(cfg config.Config) error {
	items, err := source.Scan(cfg.Source)
	if err != nil {
		return err
	}
	m := newModel(cfg, items)
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

type model struct {
	cfg     config.Config
	items   []source.Item
	entries []sync.Entry
	cursor  int
	width   int
	height  int
	status  string
}

func newModel(cfg config.Config, items []source.Item) model {
	return model{
		cfg:     cfg,
		items:   items,
		entries: sync.Inspect(cfg, items),
		status:  "ready",
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "r":
			items, err := source.Scan(m.cfg.Source)
			if err != nil {
				m.status = "scan error: " + err.Error()
				return m, nil
			}
			m.items = items
			m.entries = sync.Inspect(m.cfg, items)
			m.status = "rescanned"
		case "i":
			if m.cursor < len(m.entries) {
				e := m.entries[m.cursor]
				if _, err := sync.Install(e.Target, e.Target.ResolveStrategy(m.cfg.DefaultStrategy), e.Item); err != nil {
					m.status = "install: " + err.Error()
				} else {
					m.entries = sync.Inspect(m.cfg, m.items)
					m.status = fmt.Sprintf("installed %s -> %s", e.Item.Name, e.Target.Name)
				}
			}
		case "x":
			if m.cursor < len(m.entries) {
				e := m.entries[m.cursor]
				if err := sync.Uninstall(e.Target, e.Target.ResolveStrategy(m.cfg.DefaultStrategy), e.Item); err != nil {
					m.status = "uninstall: " + err.Error()
				} else {
					m.entries = sync.Inspect(m.cfg, m.items)
					m.status = fmt.Sprintf("removed %s from %s", e.Item.Name, e.Target.Name)
				}
			}
		}
	}
	return m, nil
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

func (m model) View() string {
	var b []byte
	b = append(b, titleStyle.Render("lazyagentcfg "+version.Version)...)
	b = append(b, '\n', '\n')

	if len(m.entries) == 0 {
		b = append(b, dimStyle.Render("no items found in "+m.cfg.Source)...)
		b = append(b, '\n')
	}

	for i, e := range m.entries {
		line := fmt.Sprintf("%-8s  %-7s  %-24s  %s", e.Target.Name, e.Item.Kind, e.Item.Name, e.Status)
		if i == m.cursor {
			b = append(b, cursorStyle.Render("▶ "+line)...)
		} else {
			b = append(b, ("  " + line)...)
		}
		b = append(b, '\n')
	}

	b = append(b, '\n')
	b = append(b, statusStyle.Render(m.status)...)
	b = append(b, '\n')
	b = append(b, dimStyle.Render("j/k move · i install · x uninstall · r rescan · q quit")...)
	return string(b)
}
