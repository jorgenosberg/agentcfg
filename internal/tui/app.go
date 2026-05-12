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

type viewMode int

const (
	viewSource   viewMode = iota // global source items vs targets
	viewProjects                 // project-local agent configuration files
)

// Run starts the lazycfg TUI.
func Run(cfg config.Config) error {
	items, err := source.Scan(cfg.Source)
	if err != nil {
		return err
	}
	projectItems := scanAllProjects(cfg)
	m := newModel(cfg, items, projectItems)
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func scanAllProjects(cfg config.Config) []source.ProjectItem {
	var all []source.ProjectItem
	for _, p := range cfg.Projects {
		items, err := source.ScanProject(p.Path, p.Name)
		if err != nil {
			continue
		}
		all = append(all, items...)
	}
	return all
}

type model struct {
	cfg          config.Config
	items        []source.Item
	entries      []sync.Entry
	projectItems []source.ProjectItem
	cursor       int
	width        int
	height       int
	status       string
	mode         viewMode
}

func newModel(cfg config.Config, items []source.Item, projectItems []source.ProjectItem) model {
	return model{
		cfg:          cfg,
		items:        items,
		entries:      sync.Inspect(cfg, items),
		projectItems: projectItems,
		status:       "ready",
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
		case "tab", "p":
			if m.mode == viewSource {
				m.mode = viewProjects
			} else {
				m.mode = viewSource
			}
			m.cursor = 0
			m.status = map[viewMode]string{
				viewSource:   "source view",
				viewProjects: "projects view",
			}[m.mode]
		case "j", "down":
			max := m.currentLen() - 1
			if m.cursor < max {
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
			m.projectItems = scanAllProjects(m.cfg)
			m.status = "rescanned"
		case "i":
			if m.mode == viewSource && m.cursor < len(m.entries) {
				e := m.entries[m.cursor]
				if _, err := sync.Install(e.Target, e.Target.ResolveStrategy(m.cfg.DefaultStrategy), e.Item); err != nil {
					m.status = "install: " + err.Error()
				} else {
					m.entries = sync.Inspect(m.cfg, m.items)
					m.status = fmt.Sprintf("installed %s -> %s", e.Item.Name, e.Target.Name)
				}
			}
		case "x":
			if m.mode == viewSource && m.cursor < len(m.entries) {
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

func (m model) currentLen() int {
	if m.mode == viewProjects {
		return len(m.projectItems)
	}
	return len(m.entries)
}

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	tabStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	tabActiveStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	cursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	statusStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

func (m model) View() string {
	var b []byte
	b = append(b, titleStyle.Render("lazyagentcfg "+version.Version)...)

	// Tab bar
	b = append(b, "  "...)
	sourceTab := tabStyle.Render("[1] source")
	projectsTab := tabStyle.Render("[2] projects")
	if m.mode == viewSource {
		sourceTab = tabActiveStyle.Render("[1] source")
	} else {
		projectsTab = tabActiveStyle.Render("[2] projects")
	}
	b = append(b, sourceTab...)
	b = append(b, "  "...)
	b = append(b, projectsTab...)
	b = append(b, '\n', '\n')

	if m.mode == viewSource {
		b = m.renderSourceView(b)
	} else {
		b = m.renderProjectsView(b)
	}

	b = append(b, '\n')
	b = append(b, statusStyle.Render(m.status)...)
	b = append(b, '\n')
	if m.mode == viewSource {
		b = append(b, dimStyle.Render("j/k move · i install · x uninstall · r rescan · tab/p projects · q quit")...)
	} else {
		b = append(b, dimStyle.Render("j/k move · r rescan · tab/p source · q quit")...)
	}
	return string(b)
}

func (m model) renderSourceView(b []byte) []byte {
	if len(m.entries) == 0 {
		b = append(b, dimStyle.Render("no items found in "+m.cfg.Source)...)
		b = append(b, '\n')
		return b
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
	return b
}

func (m model) renderProjectsView(b []byte) []byte {
	if len(m.projectItems) == 0 {
		if len(m.cfg.Projects) == 0 {
			b = append(b, dimStyle.Render("no projects configured — add one with: agentcfg project add <name> <path>")...)
		} else {
			b = append(b, dimStyle.Render("no agent configuration files found in configured projects")...)
		}
		b = append(b, '\n')
		return b
	}
	for i, it := range m.projectItems {
		line := fmt.Sprintf("%-14s  %-10s  %-7s  %-24s  %s", it.Project, it.Agent, it.Kind, it.Name, it.RelPath)
		if i == m.cursor {
			b = append(b, cursorStyle.Render("▶ "+line)...)
		} else {
			b = append(b, ("  " + line)...)
		}
		b = append(b, '\n')
	}
	return b
}
