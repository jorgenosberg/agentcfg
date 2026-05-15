package tui

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/icons"
	"github.com/jorgenosberg/agentcfg/internal/lock"
	"github.com/jorgenosberg/agentcfg/internal/source"
	"github.com/jorgenosberg/agentcfg/internal/sync"
	"github.com/jorgenosberg/agentcfg/internal/version"
)

type viewMode int

const (
	viewSource   viewMode = iota // global source items vs targets
	viewProjects                 // project-local agent configuration files
)

func Run(cfgPath string, cfg config.Config) error {
	items, err := source.Scan(cfg.Source)
	if err != nil {
		return err
	}
	projectItems := scanAllProjects(cfg)

	// Preload agent logo images while still in the normal screen so the
	// terminal has them cached before the alt screen takes over.
	icons.Preload(targetNamesFromConfig(cfg))

	m := newModel(cfgPath, cfg, items, projectItems)
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
	cfgPath      string
	cfg          config.Config
	items        []source.Item
	entries      []sync.Entry
	projectItems []source.ProjectItem
	cursor       int
	offset       int
	width        int
	height       int
	status       string
	mode         viewMode
	sourceKind   string // kind filter for source view: "" = all, else KindSkill/Hook/Context
	overlay      overlayModel
}

func newModel(cfgPath string, cfg config.Config, items []source.Item, projectItems []source.ProjectItem) model {
	return model{
		cfgPath:      cfgPath,
		cfg:          cfg,
		items:        items,
		entries:      sync.Inspect(cfg, items),
		projectItems: projectItems,
		status:       "ready",
	}
}

func (m model) Init() tea.Cmd { return nil }

// innerWidths returns the inner content widths for the left and right panels
// (excluding border characters) and whether the right panel is shown.
// Total: 3 chars of borders (│ left │ divider │ right) + leftIW + rightIW = m.width.
func (m model) innerWidths() (leftIW, rightIW int, hasRight bool) {
	if m.width == 0 {
		return 78, 0, false
	}
	innerW := m.width - 3
	leftIW = max(48, innerW*2/5)
	rightIW = innerW - leftIW
	if rightIW < 20 {
		return m.width - 2, 0, false
	}
	return leftIW, rightIW, true
}

func (m model) listHeight() int {
	if m.height == 0 {
		return 20
	}
	if h := m.height - 4; h >= 1 {
		return h
	}
	return 1
}

func (m model) filteredEntries() []sync.Entry {
	if m.sourceKind == "" {
		return m.entries
	}
	out := make([]sync.Entry, 0, len(m.entries))
	for _, e := range m.entries {
		if e.Item.Kind == m.sourceKind {
			out = append(out, e)
		}
	}
	return out
}

func (m model) currentLen() int {
	if m.mode == viewProjects {
		return len(m.projectItems)
	}
	return len(m.filteredEntries())
}

func (m model) adjustOffset() model {
	lh := m.listHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	} else if m.cursor >= m.offset+lh {
		m.offset = m.cursor - lh + 1
	}
	return m
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m = m.adjustOffset()

	case cfgReloadMsg:
		if msg.err != nil {
			m.status = "error: " + msg.err.Error()
			return m, nil
		}
		cfg, err := config.Load(m.cfgPath)
		if err != nil {
			m.status = "reload error: " + err.Error()
			return m, nil
		}
		m.cfg = cfg
		items, _ := source.Scan(cfg.Source)
		m.items = items
		m.entries = sync.Inspect(cfg, items)
		m.projectItems = scanAllProjects(cfg)
		icons.Preload(targetNamesFromConfig(cfg))
		if n := m.currentLen(); m.cursor >= n {
			m.cursor = max(0, n-1)
		}
		m = m.adjustOffset()
		m.status = "ready"

	case tea.KeyMsg:
		if m.overlay != nil {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			next, cmd := m.overlay.Update(msg)
			m.overlay = next
			return m, cmd
		}
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "?":
			m.overlay = newHelpOverlay()
		case "1":
			if m.mode != viewSource {
				m.mode = viewSource
				m.cursor, m.offset = 0, 0
				m.status = "source view"
			}
		case "2":
			if m.mode != viewProjects {
				m.mode = viewProjects
				m.cursor, m.offset = 0, 0
				m.status = "projects view"
			}
		case "tab", "p":
			if m.mode == viewSource {
				m.mode = viewProjects
			} else {
				m.mode = viewSource
			}
			m.cursor, m.offset = 0, 0
			m.status = map[viewMode]string{
				viewSource:   "source view",
				viewProjects: "projects view",
			}[m.mode]
		case "j", "down":
			if m.cursor < m.currentLen()-1 {
				m.cursor++
				m = m.adjustOffset()
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m = m.adjustOffset()
			}
		case "g", "home":
			m.cursor = 0
			m = m.adjustOffset()
		case "G", "end":
			if n := m.currentLen(); n > 0 {
				m.cursor = n - 1
				m = m.adjustOffset()
			}
		case "ctrl+u", "pgup":
			half := max(1, m.listHeight()/2)
			m.cursor = max(0, m.cursor-half)
			m = m.adjustOffset()
		case "ctrl+d", "pgdown":
			half := max(1, m.listHeight()/2)
			m.cursor = min(m.currentLen()-1, m.cursor+half)
			m = m.adjustOffset()
		case "enter":
			if m.mode == viewSource {
				entries := m.filteredEntries()
				if m.cursor < len(entries) {
					e := entries[m.cursor]
					if _, err := sync.Install(e.Target, e.Target.ResolveStrategy(m.cfg.DefaultStrategy), e.Item); err != nil {
						m.status = "install: " + err.Error()
					} else {
						m.entries = sync.Inspect(m.cfg, m.items)
						m.status = fmt.Sprintf("installed %s -> %s", e.Item.Name, e.Target.Name)
					}
				}
			}
		case "f":
			if m.mode == viewSource {
				switch m.sourceKind {
				case "":
					m.sourceKind = source.KindSkill
				case source.KindSkill:
					m.sourceKind = source.KindHook
				case source.KindHook:
					m.sourceKind = source.KindContext
				default:
					m.sourceKind = ""
				}
				m.cursor, m.offset = 0, 0
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
			if n := m.currentLen(); m.cursor >= n {
				m.cursor = max(0, n-1)
			}
			m = m.adjustOffset()
		case "I":
			o, cmd := newInitWizardOverlay(m.cfgPath)
			m.overlay = o
			return m, cmd
		case "D":
			m.overlay = newDiscoverOverlay(m.cfgPath, m.cfg)
		case "n":
			if m.mode == viewSource {
				o, cmd := newAddTargetOverlay(m.cfgPath, m.cfg)
				m.overlay = o
				return m, cmd
			}
			o, cmd := newAddProjectOverlay(m.cfgPath, m.cfg)
			m.overlay = o
			return m, cmd
		case "d":
			if m.mode == viewSource {
				entries := m.filteredEntries()
				if m.cursor < len(entries) {
					targetName := entries[m.cursor].Target.Name
					cfgPath, cfg := m.cfgPath, m.cfg
					m.overlay = newConfirmOverlay(
						fmt.Sprintf("Remove target %q?", targetName),
						"Removes from config only. Installed items are not uninstalled.",
						func() error {
							out := make([]config.Target, 0, len(cfg.Targets))
							for _, t := range cfg.Targets {
								if t.Name != targetName {
									out = append(out, t)
								}
							}
							cfg.Targets = out
							return config.Save(cfgPath, cfg)
						},
					)
				}
			} else {
				if m.cursor < len(m.projectItems) {
					projName := m.projectItems[m.cursor].Project
					cfgPath, cfg := m.cfgPath, m.cfg
					m.overlay = newConfirmOverlay(
						fmt.Sprintf("Remove project %q?", projName),
						"Removes from config only. No files are deleted.",
						func() error {
							out := make([]config.Project, 0, len(cfg.Projects))
							for _, p := range cfg.Projects {
								if p.Name != projName {
									out = append(out, p)
								}
							}
							cfg.Projects = out
							return config.Save(cfgPath, cfg)
						},
					)
				}
			}
		case "i":
			if m.mode == viewSource {
				entries := m.filteredEntries()
				if m.cursor < len(entries) {
					e := entries[m.cursor]
					if _, err := sync.Install(e.Target, e.Target.ResolveStrategy(m.cfg.DefaultStrategy), e.Item); err != nil {
						m.status = "install: " + err.Error()
					} else {
						m.entries = sync.Inspect(m.cfg, m.items)
						m.status = fmt.Sprintf("installed %s -> %s", e.Item.Name, e.Target.Name)
					}
				}
			}
		case "x":
			if m.mode == viewSource {
				entries := m.filteredEntries()
				if m.cursor < len(entries) {
					e := entries[m.cursor]
					if err := sync.Uninstall(e.Target, e.Target.ResolveStrategy(m.cfg.DefaultStrategy), e.Item); err != nil {
						m.status = "uninstall: " + err.Error()
					} else {
						m.entries = sync.Inspect(m.cfg, m.items)
						m.status = fmt.Sprintf("removed %s from %s", e.Item.Name, e.Target.Name)
					}
				}
			}
		case "S":
			if m.mode == viewSource {
				lockPath, err := lock.DefaultPath()
				if err != nil {
					m.status = "sync: " + err.Error()
					break
				}
				lck, err := lock.Load(lockPath)
				if err != nil {
					m.status = "sync: " + err.Error()
					break
				}
				results := sync.Sync(m.cfg, m.items, lck, false)
				var installed, updated int
				for _, r := range results {
					if r.Err == nil {
						if r.OldStatus == sync.StatusAbsent {
							installed++
						} else {
							updated++
						}
					}
				}
				if len(results) > 0 {
					_ = lock.Save(lockPath, lck)
				}
				m.entries = sync.Inspect(m.cfg, m.items)
				if len(results) == 0 {
					m.status = "everything up to date"
				} else {
					m.status = fmt.Sprintf("sync: %d installed, %d updated", installed, updated)
				}
			}
		}
	default:
		// Forward all other messages (e.g. cursor blink ticks) to active overlay.
		if m.overlay != nil {
			next, cmd := m.overlay.Update(msg)
			m.overlay = next
			return m, cmd
		}
	}
	return m, nil
}

var (
	tabStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	tabActiveStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	cursorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	dimStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	statusStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	borderStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	countStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	previewStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("247"))
	activeBorderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	inactiveBorderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	selectedRowStyle   = lipgloss.NewStyle().Background(lipgloss.Color("236"))

	statusLinkedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("34"))
	statusCopiedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("36"))
	statusDriftedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	statusAbsentStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	statusForeignStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

func renderStatus(s sync.Status) string {
	switch s {
	case sync.StatusLinked:
		return statusLinkedStyle.Render(string(s))
	case sync.StatusCopied:
		return statusCopiedStyle.Render(string(s))
	case sync.StatusDrifted:
		return statusDriftedStyle.Render(string(s))
	case sync.StatusAbsent:
		return statusAbsentStyle.Render(string(s))
	case sync.StatusForeign:
		return statusForeignStyle.Render(string(s))
	default:
		return string(s)
	}
}

func (m model) View() string {
	if m.overlay != nil {
		return m.overlay.View(m.width, m.height)
	}

	leftIW, rightIW, hasRight := m.innerWidths()
	lh := m.listHeight()

	var b strings.Builder

	b.WriteString(m.renderTopBorder(leftIW, rightIW, hasRight))
	b.WriteByte('\n')
	b.WriteString(m.renderFilterBoxRow(leftIW, rightIW, hasRight))
	b.WriteByte('\n')

	leftRows := m.buildContentRows(lh, leftIW)
	var rightRows []string
	if hasRight {
		rightRows = m.buildPreviewLines(lh, rightIW)
	}

	aB := activeBorderStyle.Render("│")
	iB := inactiveBorderStyle.Render("│")

	for i := range lh {
		left := ""
		if i < len(leftRows) {
			left = leftRows[i]
		}
		if hasRight {
			right := ""
			if i < len(rightRows) {
				right = rightRows[i]
			}
			b.WriteString(aB + padToWidth(left, leftIW) + iB + padToWidth(right, rightIW) + iB)
		} else {
			b.WriteString(aB + padToWidth(left, leftIW) + aB)
		}
		b.WriteByte('\n')
	}

	b.WriteString(m.renderBottomBorder(leftIW, rightIW, hasRight))
	b.WriteByte('\n')
	b.WriteString(m.renderFooter(m.width))
	return b.String()
}

func (m model) buildContentRows(lh, leftIW int) []string {
	if m.mode == viewSource {
		return m.buildSourceRows(lh, leftIW)
	}
	return m.buildProjectsRows(lh, leftIW)
}

func (m model) renderTopBorder(leftIW, rightIW int, hasRight bool) string {
	var tabs string
	if m.mode == viewSource {
		tabs = tabActiveStyle.Render("[1] Source") + borderStyle.Render(" · ") + tabStyle.Render("[2] Projects")
	} else {
		tabs = tabStyle.Render("[1] Source") + borderStyle.Render(" · ") + tabActiveStyle.Render("[2] Projects")
	}
	tabsVis := lipgloss.Width(tabs)
	const prefix = "─ "
	const suffix = " "
	padW := max(0, leftIW-len(prefix)-tabsVis-len(suffix))
	leftSec := activeBorderStyle.Render(prefix) + tabs + activeBorderStyle.Render(strings.Repeat("─", padW)+suffix)

	if !hasRight {
		return activeBorderStyle.Render("┌") + leftSec + activeBorderStyle.Render("┐")
	}
	rightLabel := "─ Preview "
	rightPadW := max(0, rightIW-len(rightLabel))
	rightSec := inactiveBorderStyle.Render(rightLabel + strings.Repeat("─", rightPadW))
	return activeBorderStyle.Render("┌") + leftSec + inactiveBorderStyle.Render("┬") + rightSec + inactiveBorderStyle.Render("┐")
}

func (m model) renderFilterBoxRow(leftIW, rightIW int, hasRight bool) string {
	var leftContent string
	if m.mode == viewSource {
		filters := []struct{ kind, label string }{
			{"", "all"},
			{source.KindSkill, "skills"},
			{source.KindHook, "hooks"},
			{source.KindContext, "context"},
		}
		sep := dimStyle.Render(" · ")
		parts := make([]string, len(filters))
		for i, f := range filters {
			if f.kind == m.sourceKind {
				parts[i] = tabActiveStyle.Render(f.label)
			} else {
				parts[i] = tabStyle.Render(f.label)
			}
		}
		leftContent = " " + strings.Join(parts, sep)
	}
	aB := activeBorderStyle.Render("│")
	if !hasRight {
		return aB + padToWidth(leftContent, leftIW) + aB
	}
	iB := inactiveBorderStyle.Render("│")
	return aB + padToWidth(leftContent, leftIW) + iB + padToWidth("", rightIW) + iB
}

func (m model) renderBottomBorder(leftIW, rightIW int, hasRight bool) string {
	total := m.currentLen()
	cur := m.cursor + 1
	if total == 0 {
		cur = 0
	}
	countStr := fmt.Sprintf(" %d of %d ", cur, total)
	padW := max(0, leftIW-len(countStr))
	leftSec := activeBorderStyle.Render(strings.Repeat("─", padW)) + countStyle.Render(countStr)

	if !hasRight {
		return activeBorderStyle.Render("└") + leftSec + activeBorderStyle.Render("┘")
	}
	rightSec := inactiveBorderStyle.Render(strings.Repeat("─", rightIW))
	return activeBorderStyle.Render("└") + leftSec + inactiveBorderStyle.Render("┴") + rightSec + inactiveBorderStyle.Render("┘")
}

const iconWidth = 4 // iconStrip always returns 4 visible chars

func (m model) buildSourceRows(lh, leftIW int) []string {
	entries := m.filteredEntries()
	rows := make([]string, 0, lh)

	if len(m.cfg.Targets) == 0 {
		rows = append(rows, dimStyle.Render("  no targets — press I for wizard, D to discover, n to add"))
		for len(rows) < lh {
			rows = append(rows, "")
		}
		return rows
	}
	if len(entries) == 0 {
		rows = append(rows, dimStyle.Render("  no items found in "+m.cfg.Source))
		for len(rows) < lh {
			rows = append(rows, "")
		}
		return rows
	}

	end := min(m.offset+lh, len(entries))
	for i := m.offset; i < end; i++ {
		e := entries[i]
		icon := iconStrip(e.Target.Name)
		statusStr := renderStatus(e.Status)
		line := fmt.Sprintf("%-8s  %-7s  %-24s  %s", e.Target.Name, e.Item.Kind, e.Item.Name, statusStr)
		if i == m.cursor {
			content := cursorStyle.Render("▌ ") + line
			rows = append(rows, icon+selectedRowStyle.Render(padToWidth(content, leftIW-iconWidth)))
		} else {
			rows = append(rows, icon+"  "+line)
		}
	}
	for len(rows) < lh {
		rows = append(rows, "")
	}
	return rows
}

func (m model) buildProjectsRows(lh, leftIW int) []string {
	rows := make([]string, 0, lh)

	if len(m.projectItems) == 0 {
		msg := "  no agent files found in configured projects"
		if len(m.cfg.Projects) == 0 {
			msg = "  no projects — add one: agentcfg project add <name> <path>"
		}
		rows = append(rows, dimStyle.Render(msg))
		for len(rows) < lh {
			rows = append(rows, "")
		}
		return rows
	}

	end := min(m.offset+lh, len(m.projectItems))
	for i := m.offset; i < end; i++ {
		it := m.projectItems[i]
		icon := iconStrip(it.Agent)
		line := fmt.Sprintf("%-14s  %-10s  %-7s  %-24s  %s", it.Project, it.Agent, it.Kind, it.Name, it.RelPath)
		if i == m.cursor {
			content := cursorStyle.Render("▌ ") + line
			rows = append(rows, icon+selectedRowStyle.Render(padToWidth(content, leftIW-iconWidth)))
		} else {
			rows = append(rows, icon+"  "+line)
		}
	}
	for len(rows) < lh {
		rows = append(rows, "")
	}
	return rows
}

func (m model) buildPreviewLines(lh, w int) []string {
	path, isDir, ok := m.currentPreviewPath()
	var raw []string
	if ok {
		raw = readPreview(path, isDir, lh, w)
	}
	lines := make([]string, lh)
	for i := range lines {
		if i < len(raw) {
			lines[i] = raw[i]
		}
	}
	return lines
}

func (m model) currentPreviewPath() (path string, isDir bool, ok bool) {
	if m.mode == viewSource {
		entries := m.filteredEntries()
		if m.cursor >= len(entries) {
			return "", false, false
		}
		e := entries[m.cursor]
		return e.Item.Path, e.Item.Kind == source.KindSkill, true
	}
	if m.cursor >= len(m.projectItems) {
		return "", false, false
	}
	it := m.projectItems[m.cursor]
	return it.Path, it.Kind == source.KindSkill, true
}

func readPreview(path string, isDir bool, maxLines, maxWidth int) []string {
	if isDir {
		entries, err := os.ReadDir(path)
		if err != nil {
			return []string{dimStyle.Render("  " + err.Error())}
		}
		lines := make([]string, 0, min(len(entries), maxLines))
		for _, e := range entries {
			if len(lines) >= maxLines {
				break
			}
			name := e.Name()
			if e.IsDir() {
				name += "/"
			}
			lines = append(lines, previewStyle.Render(" "+truncateRunes(name, maxWidth-1)))
		}
		return lines
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return []string{dimStyle.Render("  " + err.Error())}
	}

	// Binary detection via null-byte scan of the first 512 bytes.
	check := data
	if len(check) > 512 {
		check = check[:512]
	}
	if bytes.IndexByte(check, 0) >= 0 {
		return []string{dimStyle.Render("  [binary file]")}
	}

	rawLines := strings.Split(string(data), "\n")
	result := make([]string, 0, min(len(rawLines), maxLines))
	for i, line := range rawLines {
		if i >= maxLines {
			break
		}
		result = append(result, previewStyle.Render(" "+truncateRunes(line, maxWidth-1)))
	}
	return result
}

func (m model) renderFooter(w int) string {
	status := m.status
	var left string
	if status == "ready" {
		left = dimStyle.Render(" lazyagentcfg " + version.Version)
	} else {
		left = statusStyle.Render(" " + status)
	}
	var right string
	if m.mode == viewSource {
		right = dimStyle.Render("i install  x remove  S sync  n/d target  f filter  r rescan  ? help  q quit ")
	} else {
		right = dimStyle.Render("n/d project  r rescan  ? help  q quit ")
	}
	lv := lipgloss.Width(left)
	rv := lipgloss.Width(right)
	gap := max(1, w-lv-rv)
	return left + strings.Repeat(" ", gap) + right
}


func padToWidth(s string, w int) string {
	vis := lipgloss.Width(s)
	if vis >= w {
		return s
	}
	return s + strings.Repeat(" ", w-vis)
}

func truncateRunes(s string, maxR int) string {
	r := []rune(s)
	if len(r) <= maxR {
		return s
	}
	if maxR <= 0 {
		return ""
	}
	return string(r[:maxR-1]) + "…"
}

func iconStrip(agent string) string {
	const cols = 3
	if icons.Has(agent) {
		return icons.Badge(agent, cols) + " "
	}
	return strings.Repeat(" ", cols+1)
}
