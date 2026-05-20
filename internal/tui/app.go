package tui

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	chromaformatters "github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/icons"
	"github.com/jorgenosberg/agentcfg/internal/lock"
	"github.com/jorgenosberg/agentcfg/internal/source"
	"github.com/jorgenosberg/agentcfg/internal/sync"
	"github.com/jorgenosberg/agentcfg/internal/version"
)

type viewMode int

const (
	viewAgentcfg    viewMode = iota // items managed in agentcfg source
	viewAgentFolders                 // items actually in agent/target directories
	viewProjects                     // project-local agent configuration files
)

const (
	focusNone   = 0
	focusKind   = 1
	focusTarget = 2
)

func Run(cfgPath string, cfg config.Config) error {
	items, err := source.Scan(cfg.Source)
	if err != nil {
		return err
	}
	projectItems := scanAllProjects(cfg)

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
	cfgPath       string
	cfg           config.Config
	items         []source.Item
	entries       []sync.Entry
	targetEntries []sync.Entry
	projectItems  []source.ProjectItem
	cursor        int
	offset        int
	width         int
	height        int
	status        string
	mode          viewMode
	sourceKind    string // kind filter for agentcfg view: "" = all, else KindSkill/Hook/Context
	sourceTarget  string // target filter for agentcfg view: "" = all
	targetFilter  string // target filter for agent folders view: "" = all
	overlay       overlayModel
	filterFocus   int // focusNone, focusKind, or focusTarget
}

type groupedItem struct {
	Item    source.Item
	Entries []sync.Entry
}

func newModel(cfgPath string, cfg config.Config, items []source.Item, projectItems []source.ProjectItem) model {
	return model{
		cfgPath:       cfgPath,
		cfg:           cfg,
		items:         items,
		entries:       sync.Inspect(cfg, items),
		targetEntries: sync.ScanTargetDirs(cfg, items),
		projectItems:  projectItems,
		status:        "ready",
	}
}

func (m model) Init() tea.Cmd { return nil }

// innerWidths returns the inner content widths for the left and right panels
// (excluding border characters) and whether the right panel is shown.
// Total: 4 chars of borders (│left│ │right│) + leftIW + rightIW = m.width.
func (m model) innerWidths() (leftIW, rightIW int, hasRight bool) {
	if m.width == 0 {
		return 78, 0, false
	}
	innerW := m.width - 4
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
	if m.sourceKind == "" && m.sourceTarget == "" {
		return m.entries
	}
	out := make([]sync.Entry, 0, len(m.entries))
	for _, e := range m.entries {
		if m.sourceKind != "" && e.Item.Kind != m.sourceKind {
			continue
		}
		if m.sourceTarget != "" && e.Target.Name != m.sourceTarget {
			continue
		}
		out = append(out, e)
	}
	return out
}

func (m model) filteredTargetEntries() []sync.Entry {
	if m.targetFilter == "" {
		return m.targetEntries
	}
	out := make([]sync.Entry, 0, len(m.targetEntries))
	for _, e := range m.targetEntries {
		if e.Target.Name == m.targetFilter {
			out = append(out, e)
		}
	}
	return out
}

func (m model) groupedItems() []groupedItem {
	entries := m.filteredEntries()
	type keyT = string
	order := make([]keyT, 0)
	seen := make(map[keyT]int) // key -> index in result
	result := make([]groupedItem, 0)
	for _, e := range entries {
		k := e.Item.Kind + "/" + e.Item.Name
		if idx, ok := seen[k]; ok {
			result[idx].Entries = append(result[idx].Entries, e)
		} else {
			seen[k] = len(result)
			order = append(order, k)
			result = append(result, groupedItem{Item: e.Item, Entries: []sync.Entry{e}})
		}
	}
	_ = order
	return result
}

func (m model) currentLen() int {
	switch m.mode {
	case viewProjects:
		return len(m.projectItems)
	case viewAgentFolders:
		return len(m.filteredTargetEntries())
	default: // viewAgentcfg
		return len(m.groupedItems())
	}
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

var kindCycle = []string{"", source.KindSkill, source.KindHook, source.KindContext}

func nextKind(current string) string {
	for i, k := range kindCycle {
		if k == current {
			return kindCycle[(i+1)%len(kindCycle)]
		}
	}
	return ""
}

func prevKind(current string) string {
	for i, k := range kindCycle {
		if k == current {
			return kindCycle[(i-1+len(kindCycle))%len(kindCycle)]
		}
	}
	return ""
}

func nextTarget(current string, targets []config.Target) string {
	if current == "" {
		if len(targets) > 0 {
			return targets[0].Name
		}
		return ""
	}
	for i, t := range targets {
		if t.Name == current {
			if i+1 < len(targets) {
				return targets[i+1].Name
			}
			return ""
		}
	}
	return ""
}

func prevTarget(current string, targets []config.Target) string {
	if current == "" {
		if len(targets) > 0 {
			return targets[len(targets)-1].Name
		}
		return ""
	}
	for i, t := range targets {
		if t.Name == current {
			if i > 0 {
				return targets[i-1].Name
			}
			return ""
		}
	}
	return ""
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
		m.targetEntries = sync.ScanTargetDirs(cfg, items)
		m.projectItems = scanAllProjects(cfg)
		if n := m.currentLen(); m.cursor >= n {
			m.cursor = max(0, n-1)
		}
		m = m.adjustOffset()
		if msg.status != "" {
			m.status = msg.status
		} else {
			m.status = "ready"
		}

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
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.filterFocus != focusNone {
				m.filterFocus = focusNone
				return m, nil
			}
			return m, nil // no-op in main navigation
		case "?":
			m.overlay = newHelpOverlay()
		case "tab":
			switch m.mode {
			case viewAgentcfg:
				m.mode = viewAgentFolders
			case viewAgentFolders:
				m.mode = viewProjects
			default:
				m.mode = viewAgentcfg
			}
			m.cursor, m.offset = 0, 0
			m.status = map[viewMode]string{
				viewAgentcfg:    "source view",
				viewAgentFolders: "agents view",
				viewProjects:    "projects view",
			}[m.mode]
		case "j", "down":
			m.filterFocus = focusNone
			if m.cursor < m.currentLen()-1 {
				m.cursor++
				m = m.adjustOffset()
			}
		case "k", "up":
			m.filterFocus = focusNone
			if m.cursor > 0 {
				m.cursor--
				m = m.adjustOffset()
			}
		case "home", "g":
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
			if m.filterFocus != focusNone {
				m.filterFocus = focusNone
				return m, nil
			}
			actions := m.buildItemActions()
			if len(actions) > 0 {
				m.overlay = newPaletteOverlay(m.currentItemTitle(), actions)
			}
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			n := int(msg.String()[0] - '0')
			actions := m.buildItemActions()
			if n <= len(actions) {
				next, cmd := actions[n-1].fn()
				m.overlay = next
				return m, cmd
			}
		case "ctrl+p":
			m.overlay = newPaletteOverlay("Commands", m.buildGlobalActions())
		case "f":
			if m.mode == viewAgentcfg {
				m.filterFocus = focusKind
			}
			return m, nil
		case "t":
			if m.mode == viewAgentcfg || m.mode == viewAgentFolders {
				m.filterFocus = focusTarget
			}
			return m, nil
		case "left":
			if m.filterFocus == focusKind {
				m.sourceKind = prevKind(m.sourceKind)
				m.cursor, m.offset = 0, 0
				return m, nil
			}
			if m.filterFocus == focusTarget {
				switch m.mode {
				case viewAgentcfg:
					m.sourceTarget = prevTarget(m.sourceTarget, m.cfg.Targets)
				case viewAgentFolders:
					m.targetFilter = prevTarget(m.targetFilter, m.cfg.Targets)
				}
				m.cursor, m.offset = 0, 0
				return m, nil
			}
		case "right":
			if m.filterFocus == focusKind {
				m.sourceKind = nextKind(m.sourceKind)
				m.cursor, m.offset = 0, 0
				return m, nil
			}
			if m.filterFocus == focusTarget {
				switch m.mode {
				case viewAgentcfg:
					m.sourceTarget = nextTarget(m.sourceTarget, m.cfg.Targets)
				case viewAgentFolders:
					m.targetFilter = nextTarget(m.targetFilter, m.cfg.Targets)
				}
				m.cursor, m.offset = 0, 0
				return m, nil
			}
		case "r":
			items, err := source.Scan(m.cfg.Source)
			if err != nil {
				m.status = "scan error: " + err.Error()
				return m, nil
			}
			m.items = items
			m.entries = sync.Inspect(m.cfg, items)
			m.targetEntries = sync.ScanTargetDirs(m.cfg, items)
			m.projectItems = scanAllProjects(m.cfg)
			m.status = "rescanned"
			if n := m.currentLen(); m.cursor >= n {
				m.cursor = max(0, n-1)
			}
			m = m.adjustOffset()
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
	tabActiveStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("108"))
	cursorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("108"))
	dimStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	statusStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("108"))
	borderStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	countStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	previewStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("247"))
	activeBorderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("178"))
	inactiveBorderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	selectedRowStyle   = lipgloss.NewStyle().Background(lipgloss.Color("236"))

	statusLinkedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("34"))
	statusCopiedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("36"))
	statusDriftedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	statusAbsentStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	statusUnmanagedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	statusDisabledStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Faint(true)
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
	case sync.StatusUnmanaged:
		return statusUnmanagedStyle.Render(string(s))
	case sync.StatusDisabled:
		return statusDisabledStyle.Render("off")
	default:
		return string(s)
	}
}

func (m model) renderMainView() string {
	leftIW, rightIW, hasRight := m.innerWidths()
	lh := m.listHeight()
	leftLines := m.buildLeftPanel(lh, leftIW)
	var b strings.Builder
	if hasRight {
		rightLines := m.buildRightPanel(lh, rightIW)
		for i, l := range leftLines {
			b.WriteString(l)
			if i < len(rightLines) {
				b.WriteString(rightLines[i])
			}
			b.WriteByte('\n')
		}
	} else {
		for _, l := range leftLines {
			b.WriteString(l)
			b.WriteByte('\n')
		}
	}
	b.WriteString(borderStyle.Render(strings.Repeat("─", m.width)) + "\n")
	b.WriteString(m.renderFooter(m.width))
	return b.String()
}

func (m model) View() string {
	bg := m.renderMainView()
	if m.overlay == nil {
		return bg
	}
	popup := m.overlay.View(m.width)
	ppLines := strings.Split(popup, "\n")
	popupH := len(ppLines)
	popupW := 0
	for _, l := range ppLines {
		if w := lipgloss.Width(l); w > popupW {
			popupW = w
		}
	}
	x := (m.width - popupW) / 2
	y := (m.height - popupH) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return pasteOverlay(dimBackground(bg), popup, x, y)
}

func (m model) buildContentRows(lh, leftIW int) []string {
	switch m.mode {
	case viewAgentcfg:
		return m.buildGroupedRows(lh, leftIW)
	case viewAgentFolders:
		return m.buildAgentFolderRows(lh, leftIW)
	default: // viewProjects
		return m.buildProjectsRows(lh, leftIW)
	}
}

func (m model) buildLeftPanel(lh, leftIW int) []string {
	aR := activeBorderStyle.Render

	tab := func(label string, active bool) string {
		if active {
			return tabActiveStyle.Render(label)
		}
		return tabStyle.Render(label)
	}
	sep := borderStyle.Render(" · ")
	tabs := tab("Source", m.mode == viewAgentcfg) + sep +
		tab("Agents", m.mode == viewAgentFolders) + sep +
		tab("Projects", m.mode == viewProjects)
	tabsVis := lipgloss.Width(tabs)
	padW := max(0, leftIW-tabsVis-3)
	topBorder := aR("┌─ ") + tabs + aR(strings.Repeat("─", padW)+"─┐")

	buildTargetFilterContent := func(current string, focused, dimmed bool) string {
		activePill := tabActiveStyle
		inactivePill := tabStyle
		if dimmed {
			activePill = dimStyle
			inactivePill = dimStyle
		}
		parts := make([]string, 0, len(m.cfg.Targets)+1)
		if current == "" {
			parts = append(parts, activePill.Render("[all]"))
		} else {
			parts = append(parts, inactivePill.Render("all"))
		}
		for _, t := range m.cfg.Targets {
			if t.Name == current {
				parts = append(parts, activePill.Render("["+t.Name+"]"))
			} else {
				parts = append(parts, inactivePill.Render(t.Name))
			}
		}
		prefix := "  "
		hint := ""
		if focused {
			prefix = activeBorderStyle.Render("▌ ")
			hint = activeBorderStyle.Render("  ← →")
		}
		return prefix + strings.Join(parts, dimStyle.Render(" · ")) + hint
	}

	sepRow := aR("│") + aR(strings.Repeat("─", leftIW)) + aR("│")

	buildBottom := func() string {
		total := m.currentLen()
		cur := m.cursor + 1
		if total == 0 {
			cur = 0
		}
		countStr := fmt.Sprintf(" %d of %d ", cur, total)
		padW2 := max(0, leftIW-lipgloss.Width(countStr))
		return aR("└") + aR(strings.Repeat("─", padW2)) + countStyle.Render(countStr) + aR("┘")
	}

	switch m.mode {
	case viewAgentcfg:
		filters := []struct{ kind, label string }{
			{"", "all"},
			{source.KindSkill, "skills"},
			{source.KindHook, "hooks"},
			{source.KindContext, "context"},
		}
		kindFocused := m.filterFocus == focusKind
		kindDimmed := m.filterFocus == focusTarget
		parts := make([]string, len(filters))
		for i, f := range filters {
			if f.kind == m.sourceKind {
				if kindDimmed {
					parts[i] = dimStyle.Render("[" + f.label + "]")
				} else {
					parts[i] = tabActiveStyle.Render("[" + f.label + "]")
				}
			} else {
				if kindDimmed {
					parts[i] = dimStyle.Render(f.label)
				} else {
					parts[i] = tabStyle.Render(f.label)
				}
			}
		}
		kindPrefix := "  "
		kindFocusHint := ""
		if kindFocused {
			kindPrefix = activeBorderStyle.Render("▌ ")
			kindFocusHint = activeBorderStyle.Render("  ← →")
		}
		kindFilterContent := kindPrefix + strings.Join(parts, dimStyle.Render(" · ")) + kindFocusHint
		kindFilterRow := aR("│") + padToWidth(kindFilterContent, leftIW) + aR("│")
		targetFilterRow := aR("│") + padToWidth(buildTargetFilterContent(m.sourceTarget, m.filterFocus == focusTarget, kindFocused), leftIW) + aR("│")
		badgesHdr := m.renderBadgeHeader(leftIW)
		badgesHdrVis := lipgloss.Width(badgesHdr)
		nameHdrMax := max(4, leftIW-2-7-2-2-badgesHdrVis)
		dimHdrPart := dimStyle.Render("  " + fmt.Sprintf("%-7s  ", "TYPE") + padToWidth("NAME", nameHdrMax) + "  ")
		headerRow := aR("│") + padToWidth(dimHdrPart+badgesHdr, leftIW) + aR("│")
		contentRows := m.buildContentRows(max(0, lh-3), leftIW)
		lines := make([]string, 0, lh+6)
		lines = append(lines, topBorder, kindFilterRow, targetFilterRow, sepRow, headerRow)
		for _, row := range contentRows {
			lines = append(lines, aR("│")+padToWidth(row, leftIW)+aR("│"))
		}
		lines = append(lines, buildBottom())
		return lines

	case viewAgentFolders:
		targetFilterRow := aR("│") + padToWidth(buildTargetFilterContent(m.targetFilter, m.filterFocus == focusTarget, false), leftIW) + aR("│")
		headerContent := "  " + fmt.Sprintf("%-8s  %-7s  %-24s  %s", "AGENT", "TYPE", "NAME", "STATUS")
		headerRow := aR("│") + padToWidth(dimStyle.Render(headerContent), leftIW) + aR("│")
		contentRows := m.buildContentRows(max(0, lh-2), leftIW)
		lines := make([]string, 0, lh+5)
		lines = append(lines, topBorder, targetFilterRow, sepRow, headerRow)
		for _, row := range contentRows {
			lines = append(lines, aR("│")+padToWidth(row, leftIW)+aR("│"))
		}
		lines = append(lines, buildBottom())
		return lines

	default: // viewProjects
		emptyRow := aR("│") + padToWidth("", leftIW) + aR("│")
		headerContent := "  " + fmt.Sprintf("%-16s%-10s  %-7s  %-24s  %s", "PROJECT", "AGENT", "TYPE", "NAME", "PATH")
		headerRow := aR("│") + padToWidth(dimStyle.Render(headerContent), leftIW) + aR("│")
		contentRows := m.buildContentRows(max(0, lh-2), leftIW)
		lines := make([]string, 0, lh+5)
		lines = append(lines, topBorder, emptyRow, sepRow, headerRow)
		for _, row := range contentRows {
			lines = append(lines, aR("│")+padToWidth(row, leftIW)+aR("│"))
		}
		lines = append(lines, buildBottom())
		return lines
	}
}

func (m model) buildRightPanel(lh, rightIW int) []string {
	iR := inactiveBorderStyle.Render
	total := lh + 3
	label := "─ Preview "
	if path, _, ok := m.currentPreviewPath(); ok {
		name := filepath.Base(path)
		// Reserve 4 cells: "─ " prefix (2) + trailing space (1) + closing "┐" (1).
		maxLen := rightIW - 4
		if maxLen > 0 && len([]rune(name)) > maxLen {
			name = string([]rune(name)[:maxLen])
		}
		if maxLen > 0 {
			label = "─ " + name + " "
		}
	}
	padW := max(0, rightIW-lipgloss.Width(label))
	topBorder := iR("┌") + iR(label+strings.Repeat("─", padW)) + iR("┐")
	previewLines := m.buildPreviewLines(lh+1, rightIW)
	bottomBorder := iR("└") + iR(strings.Repeat("─", rightIW)) + iR("┘")
	lines := make([]string, 0, total)
	lines = append(lines, topBorder)
	for _, row := range previewLines {
		lines = append(lines, iR("│")+padToWidth(row, rightIW)+iR("│"))
	}
	lines = append(lines, bottomBorder)
	return lines
}

// abbrev3 extracts a 3-character abbreviation: first char, then the next two
// consonants. Falls back to vowels if not enough consonants.
func abbrev3(name string) string {
	runes := []rune(name)
	if len(runes) == 0 {
		return "?"
	}
	vowels := map[rune]bool{'a': true, 'e': true, 'i': true, 'o': true, 'u': true}
	result := []rune{[]rune(strings.ToUpper(string(runes[0])))[0]}
	// consonants first
	for i := 1; i < len(runes) && len(result) < 3; i++ {
		if !vowels[[]rune(strings.ToLower(string(runes[i])))[0]] {
			result = append(result, runes[i])
		}
	}
	// fill with vowels if needed
	for i := 1; i < len(runes) && len(result) < 3; i++ {
		if vowels[[]rune(strings.ToLower(string(runes[i])))[0]] {
			result = append(result, runes[i])
		}
	}
	return string(result)
}

// badgeStyle returns 0=full name, 1=3-char abbrev, 2=single char.
func (m model) badgeStyle(leftIW int) int {
	nT := len(m.cfg.Targets)
	if nT == 0 {
		return 2
	}
	const overhead, nameMin = 13, 16
	maxBadgeW := leftIW - overhead - nameMin
	fullW := nT - 1
	for _, t := range m.cfg.Targets {
		fullW += len([]rune(t.Name))
	}
	if fullW <= maxBadgeW {
		return 0
	}
	if nT*4-1 <= maxBadgeW {
		return 1
	}
	return 2
}

func badgeLabel(name string, style int) string {
	switch style {
	case 0:
		return name
	case 1:
		return abbrev3(name)
	default:
		r := []rune(name)
		if len(r) == 0 {
			return "?"
		}
		return strings.ToUpper(string(r[0]))
	}
}

func badgeCellW(name string, style int) int {
	switch style {
	case 0:
		return len([]rune(name))
	case 1:
		return 3
	default:
		return 1
	}
}

func (m model) renderBadgeHeader(leftIW int) string {
	style := m.badgeStyle(leftIW)
	parts := make([]string, 0, len(m.cfg.Targets))
	for _, t := range m.cfg.Targets {
		label := badgeLabel(t.Name, style)
		hex := icons.BrandColor(t.Name)
		if hex != "" {
			parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Bold(true).Render(label))
		} else {
			parts = append(parts, dimStyle.Render(label))
		}
	}
	return strings.Join(parts, " ")
}

func (m model) renderBadgeCells(g groupedItem, leftIW int) string {
	style := m.badgeStyle(leftIW)
	byTarget := make(map[string]sync.Entry, len(g.Entries))
	for _, e := range g.Entries {
		byTarget[e.Target.Name] = e
	}
	parts := make([]string, 0, len(m.cfg.Targets))
	for _, t := range m.cfg.Targets {
		cw := badgeCellW(t.Name, style)
		e, ok := byTarget[t.Name]
		if ok && e.Status == sync.StatusNotApplicable {
			parts = append(parts, padToWidth("", cw))
			continue
		}
		var sym string
		var st lipgloss.Style
		if !ok || e.Status == sync.StatusAbsent {
			sym, st = "─", dimStyle
		} else {
			switch e.Status {
			case sync.StatusLinked, sync.StatusCopied:
				sym = "✓"
				hex := icons.BrandColor(t.Name)
				if hex != "" {
					st = lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Bold(true)
				} else {
					st = statusLinkedStyle
				}
			case sync.StatusDrifted:
				sym, st = "~", statusDriftedStyle
			case sync.StatusUnmanaged:
				sym, st = "!", statusUnmanagedStyle
			case sync.StatusDisabled:
				sym, st = "○", statusDisabledStyle
			default:
				sym, st = "─", dimStyle
			}
		}
		parts = append(parts, padToWidth(st.Render(sym), cw))
	}
	return strings.Join(parts, " ")
}

func (m model) buildGroupedRows(lh, leftIW int) []string {
	rows := make([]string, 0, lh)

	if len(m.cfg.Targets) == 0 {
		rows = append(rows, dimStyle.Render("  no targets — press I for wizard, D to discover, n to add"))
		for len(rows) < lh {
			rows = append(rows, "")
		}
		return rows
	}

	grouped := m.groupedItems()
	if len(grouped) == 0 {
		rows = append(rows, dimStyle.Render("  no items found in "+m.cfg.Source))
		for len(rows) < lh {
			rows = append(rows, "")
		}
		return rows
	}

	end := min(m.offset+lh, len(grouped))
	for i := m.offset; i < end; i++ {
		g := grouped[i]
		cells := m.renderBadgeCells(g, leftIW)
		cellsVis := lipgloss.Width(cells)
		kindStr := fmt.Sprintf("%-7s", g.Item.Kind)
		// cursor(2) + kind(7) + "  "(2) + name + "  "(2) + cells = leftIW
		nameMax := max(4, leftIW-2-7-2-2-cellsVis)
		name := padToWidth(truncateRunes(g.Item.Name, nameMax), nameMax)
		if i == m.cursor {
			content := cursorStyle.Render("▌ ") + kindStr + "  " + name + "  " + cells
			rows = append(rows, selectedRowStyle.Render(padToWidth(content, leftIW)))
		} else {
			rows = append(rows, "  "+kindStr+"  "+name+"  "+cells)
		}
	}
	for len(rows) < lh {
		rows = append(rows, "")
	}
	return rows
}

func (m model) buildAgentFolderRows(lh, leftIW int) []string {
	rows := make([]string, 0, lh)
	entries := m.filteredTargetEntries()

	if len(m.cfg.Targets) == 0 {
		rows = append(rows, dimStyle.Render("  no targets — press I for wizard, D to discover, n to add"))
		for len(rows) < lh {
			rows = append(rows, "")
		}
		return rows
	}
	if len(entries) == 0 {
		rows = append(rows, dimStyle.Render("  no items found in agent folders"))
		for len(rows) < lh {
			rows = append(rows, "")
		}
		return rows
	}

	end := min(m.offset+lh, len(entries))
	for i := m.offset; i < end; i++ {
		e := entries[i]
		styledAgent := agentNameStyled(e.Target.Name, 8)
		kindStr := fmt.Sprintf("%-7s", e.Item.Kind)
		statusStr := renderStatus(e.Status)
		statusVis := lipgloss.Width(statusStr)
		// cursor(2) + agent(8) + "  "(2) + kind(7) + "  "(2) + name + "  "(2) + status = leftIW
		nameMax := max(4, leftIW-2-8-2-7-2-2-statusVis)
		name := padToWidth(truncateRunes(e.Item.Name, nameMax), nameMax)
		if i == m.cursor {
			content := cursorStyle.Render("▌ ") + styledAgent + "  " + kindStr + "  " + name + "  " + statusStr
			rows = append(rows, selectedRowStyle.Render(padToWidth(content, leftIW)))
		} else {
			rows = append(rows, "  "+styledAgent+"  "+kindStr+"  "+name+"  "+statusStr)
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
		projPart := fmt.Sprintf("%-14s  ", it.Project)
		styledAgent := agentNameStyled(it.Agent, 10)
		rest := fmt.Sprintf("  %-7s  %-24s  %s", it.Kind, it.Name, it.RelPath)
		if i == m.cursor {
			content := cursorStyle.Render("▌ ") + projPart + styledAgent + rest
			rows = append(rows, selectedRowStyle.Render(padToWidth(content, leftIW)))
		} else {
			rows = append(rows, "  "+projPart+styledAgent+rest)
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
	switch m.mode {
	case viewAgentcfg:
		grouped := m.groupedItems()
		if m.cursor >= len(grouped) {
			return "", false, false
		}
		item := grouped[m.cursor].Item
		if item.Kind == source.KindSkill {
			return filepath.Join(item.Path, "SKILL.md"), false, true
		}
		return item.Path, false, true
	case viewAgentFolders:
		entries := m.filteredTargetEntries()
		if m.cursor >= len(entries) {
			return "", false, false
		}
		e := entries[m.cursor]
		if e.Item.Kind == source.KindSkill {
			return filepath.Join(e.Dest, "SKILL.md"), false, true
		}
		return e.Dest, false, true
	default: // viewProjects
		if m.cursor >= len(m.projectItems) {
			return "", false, false
		}
		it := m.projectItems[m.cursor]
		if it.Kind == source.KindSkill {
			return filepath.Join(it.Path, "SKILL.md"), false, true
		}
		return it.Path, false, true
	}
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

	if lines := syntaxHighlight(path, data, maxLines, maxWidth); lines != nil {
		return lines
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

func syntaxHighlight(path string, data []byte, maxLines, maxWidth int) []string {
	lexer := lexers.Match(path)
	if lexer == nil {
		lexer = lexers.Analyse(string(data))
	}
	if lexer == nil {
		return nil
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get("monokai")
	if style == nil {
		return nil
	}

	tokens, err := lexer.Tokenise(nil, string(data))
	if err != nil {
		return nil
	}
	var buf bytes.Buffer
	if err := chromaformatters.TTY16m.Format(&buf, style, tokens); err != nil {
		return nil
	}

	const reset = "\033[0m"
	rawLines := strings.Split(buf.String(), "\n")
	result := make([]string, 0, min(len(rawLines), maxLines))
	for i, line := range rawLines {
		if i >= maxLines {
			break
		}
		if lipgloss.Width(line) > maxWidth-1 {
			line = ansi.Truncate(line, maxWidth-1, "")
		}
		result = append(result, " "+line+reset)
	}
	return result
}

func (m model) currentItemTitle() string {
	switch m.mode {
	case viewAgentcfg:
		grouped := m.groupedItems()
		if m.cursor < len(grouped) {
			g := grouped[m.cursor]
			return g.Item.Kind + " · " + g.Item.Name
		}
	case viewAgentFolders:
		entries := m.filteredTargetEntries()
		if m.cursor < len(entries) {
			e := entries[m.cursor]
			return e.Target.Name + " · " + e.Item.Name
		}
	case viewProjects:
		if m.cursor < len(m.projectItems) {
			it := m.projectItems[m.cursor]
			return it.Project + " · " + it.Name
		}
	}
	return "Actions"
}

func (m model) buildItemActions() []paletteAction {
	switch m.mode {
	case viewAgentcfg:
		return m.buildSourceItemActions()
	case viewAgentFolders:
		return m.buildAgentItemActions()
	case viewProjects:
		return m.buildProjectItemActions()
	}
	return nil
}

func (m model) buildSourceItemActions() []paletteAction {
	grouped := m.groupedItems()
	if m.cursor >= len(grouped) {
		return nil
	}
	g := grouped[m.cursor]
	cfg := m.cfg
	cfgPath := m.cfgPath

	var hasAbsent, hasLinkedCopied, hasDrifted, hasUnmanaged bool
	for _, e := range g.Entries {
		switch e.Status {
		case sync.StatusAbsent:
			hasAbsent = true
		case sync.StatusLinked, sync.StatusCopied:
			hasLinkedCopied = true
		case sync.StatusDrifted:
			hasDrifted = true
			hasLinkedCopied = true
		case sync.StatusUnmanaged:
			hasUnmanaged = true
		}
	}

	var actions []paletteAction

	if hasAbsent || hasDrifted {
		entries := g.Entries
		item := g.Item
		actions = append(actions, paletteAction{
			label: "Install to all targets",
			fn: func() (overlayModel, tea.Cmd) {
				return nil, func() tea.Msg {
					var ok, fail int
					for _, e := range entries {
						if _, err := sync.Install(e.Target, e.Target.ResolveStrategy(cfg.DefaultStrategy), e.Item); err != nil {
							fail++
						} else {
							ok++
						}
					}
					if fail > 0 {
						return cfgReloadMsg{status: fmt.Sprintf("installed %d, %d errors", ok, fail)}
					}
					return cfgReloadMsg{status: fmt.Sprintf("installed %s (%d targets)", item.Name, ok)}
				}
			},
		})
	}

	// Toggle — always offered
	{
		item := g.Item
		targets := cfg.Targets
		if m.sourceTarget != "" {
			for _, t := range cfg.Targets {
				if t.Name == m.sourceTarget {
					targets = []config.Target{t}
					break
				}
			}
		}
		allDisabled := true
		for _, t := range targets {
			if !t.IsDisabled(item) {
				allDisabled = false
				break
			}
		}
		disable := !allDisabled
		label := "Disable item"
		if allDisabled {
			label = "Enable item"
		}
		tgts := targets
		actions = append(actions, paletteAction{
			label: label,
			fn: func() (overlayModel, tea.Cmd) {
				return nil, func() tea.Msg {
					for _, t := range tgts {
						if err := sync.Toggle(cfgPath, t.Name, item, disable); err != nil {
							return cfgReloadMsg{err: err}
						}
					}
					return cfgReloadMsg{status: fmt.Sprintf("toggled %s", item.Name)}
				}
			},
		})
	}

	if hasUnmanaged {
		entries := g.Entries
		item := g.Item
		actions = append(actions, paletteAction{
			label: "Adopt unmanaged file",
			fn: func() (overlayModel, tea.Cmd) {
				return nil, func() tea.Msg {
					var ok, fail int
					for _, e := range entries {
						if e.Status != sync.StatusUnmanaged {
							continue
						}
						if _, err := sync.Adopt(e.Target, e.Target.ResolveStrategy(cfg.DefaultStrategy), e.Item); err != nil {
							fail++
						} else {
							ok++
						}
					}
					if fail > 0 {
						return cfgReloadMsg{status: fmt.Sprintf("adopted %d, %d errors", ok, fail)}
					}
					return cfgReloadMsg{status: fmt.Sprintf("adopted %s (%d targets)", item.Name, ok)}
				}
			},
		})
	}

	if hasLinkedCopied {
		entries := g.Entries
		item := g.Item
		actions = append(actions, paletteAction{
			label: "Uninstall from all targets",
			fn: func() (overlayModel, tea.Cmd) {
				return newConfirmOverlay(
					fmt.Sprintf("Uninstall %q?", item.Name),
					"Removes installed files from all targets.",
					func() error {
						var lastErr error
						for _, e := range entries {
							if err := sync.Uninstall(e.Target, e.Target.ResolveStrategy(cfg.DefaultStrategy), e.Item); err != nil {
								lastErr = err
							}
						}
						return lastErr
					},
				), nil
			},
		})

		targets := cfg.Targets
		if m.sourceTarget != "" {
			for _, t := range cfg.Targets {
				if t.Name == m.sourceTarget {
					targets = []config.Target{t}
					break
				}
			}
		}
		tgts := targets
		targetCount := len(tgts)
		detail := fmt.Sprintf("Place a real copy in %d target dir(s) and stop managing. File stays in source.", targetCount)
		if targetCount == 1 {
			detail = fmt.Sprintf("Place a real copy in %s and stop managing. File stays in source.", tgts[0].Path)
		}
		actions = append(actions, paletteAction{
			label: "Unmanage",
			fn: func() (overlayModel, tea.Cmd) {
				return newConfirmOverlay(
					fmt.Sprintf("Unmanage %q?", item.Name),
					detail,
					func() error {
						for _, t := range tgts {
							if err := sync.Unmanage(t, t.ResolveStrategy(cfg.DefaultStrategy), item); err != nil {
								return err
							}
							if err := sync.Toggle(cfgPath, t.Name, item, true); err != nil {
								return err
							}
						}
						return nil
					},
				), nil
			},
		})
	}

	return actions
}

func (m model) buildAgentItemActions() []paletteAction {
	entries := m.filteredTargetEntries()
	if m.cursor >= len(entries) {
		return nil
	}
	e := entries[m.cursor]
	dest := e.Dest

	if e.Status == sync.StatusUnmanaged {
		return []paletteAction{{
			label: "Delete (not managed by agentcfg)",
			fn: func() (overlayModel, tea.Cmd) {
				return newConfirmOverlay(
					fmt.Sprintf("Delete %q from %s?", e.Item.Name, e.Target.Name),
					fmt.Sprintf("Not managed by agentcfg. Permanently deletes:\n%s", dest),
					func() error { return os.RemoveAll(dest) },
				), nil
			},
		}}
	}

	return []paletteAction{{
		label: fmt.Sprintf("Remove from %s", e.Target.Name),
		fn: func() (overlayModel, tea.Cmd) {
			return newConfirmOverlay(
				fmt.Sprintf("Remove %q from %s?", e.Item.Name, e.Target.Name),
				fmt.Sprintf("Removes installed file:\n%s", dest),
				func() error { return os.RemoveAll(dest) },
			), nil
		},
	}}
}

func (m model) buildProjectItemActions() []paletteAction {
	if m.cursor >= len(m.projectItems) {
		return nil
	}
	projName := m.projectItems[m.cursor].Project
	cfgPath := m.cfgPath
	cfg := m.cfg

	return []paletteAction{{
		label: fmt.Sprintf("Remove project %q from config", projName),
		fn: func() (overlayModel, tea.Cmd) {
			return newConfirmOverlay(
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
			), nil
		},
	}}
}

func (m model) buildGlobalActions() []paletteAction {
	cfg := m.cfg
	cfgPath := m.cfgPath
	items := m.items

	var actions []paletteAction

	// Sync all
	actions = append(actions, paletteAction{
		label: "Sync all",
		fn: func() (overlayModel, tea.Cmd) {
			return nil, func() tea.Msg {
				lockPath, err := lock.DefaultPath()
				if err != nil {
					return cfgReloadMsg{err: err}
				}
				lck, err := lock.Load(lockPath)
				if err != nil {
					return cfgReloadMsg{err: err}
				}
				results := sync.Sync(cfg, items, lck, false, false)
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
				status := "everything up to date"
				if len(results) > 0 {
					status = fmt.Sprintf("sync: %d installed, %d updated", installed, updated)
				}
				return cfgReloadMsg{status: status}
			}
		},
	})

	// Rescan
	actions = append(actions, paletteAction{
		label: "Rescan",
		fn: func() (overlayModel, tea.Cmd) {
			return nil, func() tea.Msg { return cfgReloadMsg{status: "rescanned"} }
		},
	})

	// Add target
	actions = append(actions, paletteAction{
		label: "Add target",
		fn: func() (overlayModel, tea.Cmd) {
			o, cmd := newAddTargetOverlay(cfgPath, cfg)
			return o, cmd
		},
	})

	// Discover agents
	actions = append(actions, paletteAction{
		label: "Discover agents",
		fn: func() (overlayModel, tea.Cmd) {
			return newDiscoverOverlay(cfgPath, cfg), nil
		},
	})

	// Remove target — only shown if a target filter is currently active
	targetName := m.sourceTarget
	if targetName == "" {
		targetName = m.targetFilter
	}
	if targetName != "" {
		t := targetName
		actions = append(actions, paletteAction{
			label: fmt.Sprintf("Remove target %q", t),
			fn: func() (overlayModel, tea.Cmd) {
				return newConfirmOverlay(
					fmt.Sprintf("Remove target %q?", t),
					"Removes from config only. Installed items are not uninstalled.",
					func() error {
						out := make([]config.Target, 0, len(cfg.Targets))
						for _, tgt := range cfg.Targets {
							if tgt.Name != t {
								out = append(out, tgt)
							}
						}
						cfg.Targets = out
						return config.Save(cfgPath, cfg)
					},
				), nil
			},
		})
	}

	// Add project
	actions = append(actions, paletteAction{
		label: "Add project",
		fn: func() (overlayModel, tea.Cmd) {
			o, cmd := newAddProjectOverlay(cfgPath, cfg)
			return o, cmd
		},
	})

	// Remove project — only shown when cursor is on a project item
	if m.mode == viewProjects && m.cursor < len(m.projectItems) {
		projName := m.projectItems[m.cursor].Project
		p := projName
		actions = append(actions, paletteAction{
			label: fmt.Sprintf("Remove project %q", p),
			fn: func() (overlayModel, tea.Cmd) {
				return newConfirmOverlay(
					fmt.Sprintf("Remove project %q?", p),
					"Removes from config only. No files are deleted.",
					func() error {
						out := make([]config.Project, 0, len(cfg.Projects))
						for _, proj := range cfg.Projects {
							if proj.Name != p {
								out = append(out, proj)
							}
						}
						cfg.Projects = out
						return config.Save(cfgPath, cfg)
					},
				), nil
			},
		})
	}

	// Init wizard
	actions = append(actions, paletteAction{
		label: "Init wizard",
		fn: func() (overlayModel, tea.Cmd) {
			o, cmd := newInitWizardOverlay(cfgPath)
			return o, cmd
		},
	})

	return actions
}

func paletteHintKey() string {
	return "Ctrl+P"
}

func (m model) renderFooter(w int) string {
	var left string
	if m.status == "ready" {
		left = dimStyle.Render(" lazyagentcfg " + version.Version)
	} else {
		left = statusStyle.Render(" " + m.status)
	}

	var right string
	if m.filterFocus != focusNone {
		right = dimStyle.Render("← → cycle · f/t switch filter · Esc back to list ")
	} else {
		right = dimStyle.Render("↑↓ navigate · Enter actions · " + paletteHintKey() + " commands · ? help · q quit ")
	}

	lv := lipgloss.Width(left)
	rv := lipgloss.Width(right)
	gap := max(1, w-lv-rv)
	return left + strings.Repeat(" ", gap) + right
}


func agentNameStyled(agent string, fieldW int) string {
	padded := fmt.Sprintf("%-*s", fieldW, agent)
	hex := icons.BrandColor(agent)
	if hex == "" {
		return padded
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Bold(true).Render(padded)
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

