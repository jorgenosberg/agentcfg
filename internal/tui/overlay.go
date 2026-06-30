package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// dimBackground strips ANSI codes from s and re-renders every line in gray,
// producing a visible but clearly inactive background for overlay compositing.
func dimBackground(s string) string {
	gray := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = gray.Render(ansi.Strip(line))
	}
	return strings.Join(lines, "\n")
}

// pasteOverlay composites popup on top of background at position (x, y).
// It strips ANSI from each affected background line before rune-slicing, then
// re-applies gray styling to the left and right portions. Assumes ASCII/Latin
// content — rune count equals display width. popup is the raw box string from
// an overlayModel's View method.
func pasteOverlay(background, popup string, x, y int) string {
	bgLines := strings.Split(background, "\n")
	ppLines := strings.Split(popup, "\n")
	gray := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	col := x
	if col < 0 {
		col = 0
	}
	for i, ppLine := range ppLines {
		idx := y + i
		if idx < 0 || idx >= len(bgLines) {
			continue
		}
		plain := []rune(ansi.Strip(bgLines[idx]))
		ppW := lipgloss.Width(ppLine)
		for len(plain) < col+ppW {
			plain = append(plain, ' ')
		}
		left := string(plain[:col])
		right := string(plain[col+ppW:])
		bgLines[idx] = gray.Render(left) + ppLine + gray.Render(right)
	}
	return strings.Join(bgLines, "\n")
}

// cfgReloadMsg is dispatched when any management operation completes.
type cfgReloadMsg struct {
	err    error
	status string // optional status shown in footer; "ready" used if empty
}

// overlayModel is the in-TUI overlay interface. Returning nil from Update dismisses the overlay.
type overlayModel interface {
	Update(tea.Msg) (overlayModel, tea.Cmd)
	View(w int) string
}

func renderOverlayBox(w int, content, title string, boxW int) string {
	if boxW > w-4 {
		boxW = w - 4
	}
	if boxW < 20 {
		boxW = 20
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("15")).
		Padding(1, 2).
		Width(boxW).
		Render(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Render(title) + "\n\n" + content)
}

// ── multiSelect widget ────────────────────────────────────────────────────────

type msItem struct {
	label    string
	value    string
	selected bool
}

type multiSelect struct {
	items   []msItem
	cursor  int
	offset  int
	maxRows int
}

func newMultiSelect(items []msItem, maxRows int) multiSelect {
	if maxRows < 3 {
		maxRows = 3
	}
	return multiSelect{items: items, maxRows: maxRows}
}

func (ms *multiSelect) adjustOffset() {
	vis := min(ms.maxRows, len(ms.items))
	if ms.cursor < ms.offset {
		ms.offset = ms.cursor
	} else if ms.cursor >= ms.offset+vis {
		ms.offset = ms.cursor - vis + 1
	}
}

func (ms *multiSelect) handleKey(key string) {
	switch key {
	case "j", "down":
		if ms.cursor < len(ms.items)-1 {
			ms.cursor++
			ms.adjustOffset()
		}
	case "k", "up":
		if ms.cursor > 0 {
			ms.cursor--
			ms.adjustOffset()
		}
	case " ":
		if ms.cursor < len(ms.items) {
			ms.items[ms.cursor].selected = !ms.items[ms.cursor].selected
		}
	case "ctrl+a":
		allSel := true
		for _, it := range ms.items {
			if !it.selected {
				allSel = false
				break
			}
		}
		for i := range ms.items {
			ms.items[i].selected = !allSel
		}
	}
}

func (ms multiSelect) selectedItems() []msItem {
	var out []msItem
	for _, it := range ms.items {
		if it.selected {
			out = append(out, it)
		}
	}
	return out
}

func (ms multiSelect) View() string {
	vis := min(ms.maxRows, len(ms.items))
	end := min(ms.offset+vis, len(ms.items))

	var sb strings.Builder
	for i := ms.offset; i < end; i++ {
		it := ms.items[i]
		check := dimStyle.Render("[ ]")
		if it.selected {
			check = tabActiveStyle.Render("[x]")
		}
		if i == ms.cursor {
			fmt.Fprintf(&sb, "%s%s %s\n", cursorStyle.Render("▶ "), check, it.label)
		} else {
			fmt.Fprintf(&sb, "  %s %s\n", check, it.label)
		}
	}
	if len(ms.items) > vis {
		fmt.Fprintf(&sb, "%s\n", dimStyle.Render(fmt.Sprintf("  … %d items total", len(ms.items))))
	}
	return strings.TrimRight(sb.String(), "\n")
}

// ── paletteOverlay ────────────────────────────────────────────────────────────

type paletteAction struct {
	label string
	fn    func() (overlayModel, tea.Cmd)
}

type paletteWidget struct {
	actions []paletteAction
	cursor  int
}

func (pw *paletteWidget) moveUp() {
	if pw.cursor > 0 {
		pw.cursor--
	}
}

func (pw *paletteWidget) moveDown() {
	if pw.cursor < len(pw.actions)-1 {
		pw.cursor++
	}
}

func (pw *paletteWidget) selected() *paletteAction {
	if len(pw.actions) == 0 {
		return nil
	}
	return &pw.actions[pw.cursor]
}

func (pw *paletteWidget) actionAt(n int) *paletteAction {
	if n < 1 || n > len(pw.actions) {
		return nil
	}
	return &pw.actions[n-1]
}

func (pw paletteWidget) View() string {
	var sb strings.Builder
	for i, a := range pw.actions {
		num := dimStyle.Render(fmt.Sprintf(" %d  ", i+1))
		if i == pw.cursor {
			fmt.Fprintf(&sb, "%s%s%s\n", cursorStyle.Render("▶"), num, tabActiveStyle.Render(a.label))
		} else {
			fmt.Fprintf(&sb, " %s%s\n", num, a.label)
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

type paletteOverlay struct {
	title  string
	widget paletteWidget
}

func newPaletteOverlay(title string, actions []paletteAction) *paletteOverlay {
	return &paletteOverlay{title: title, widget: paletteWidget{actions: actions}}
}

func (o *paletteOverlay) Update(msg tea.Msg) (overlayModel, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return o, nil
	}
	switch key.String() {
	case "ctrl+c":
		return nil, tea.Quit
	case "esc":
		return nil, nil
	case "j", "down":
		o.widget.moveDown()
	case "k", "up":
		o.widget.moveUp()
	case "enter":
		if a := o.widget.selected(); a != nil {
			return a.fn()
		}
	default:
		s := key.String()
		if len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
			if a := o.widget.actionAt(int(s[0] - '0')); a != nil {
				return a.fn()
			}
		}
	}
	return o, nil
}

func (o *paletteOverlay) View(w int) string {
	if len(o.widget.actions) == 0 {
		content := dimStyle.Render("No actions available for this item.") +
			"\n\n" + dimStyle.Render("Esc  close")
		return renderOverlayBox(w, content, o.title, 44)
	}
	content := o.widget.View() +
		"\n\n" + dimStyle.Render("↑↓ navigate · Enter select · Esc cancel")
	return renderOverlayBox(w, content, o.title, 52)
}

// ── helpOverlay ───────────────────────────────────────────────────────────────

type helpOverlay struct{}

func newHelpOverlay() *helpOverlay { return &helpOverlay{} }

func (o *helpOverlay) Update(msg tea.Msg) (overlayModel, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return o, nil
	}
	switch key.String() {
	case "ctrl+c":
		return nil, tea.Quit
	case "q", "esc", "?":
		return nil, nil
	}
	return o, nil
}

func (o *helpOverlay) View(w int) string {
	type binding struct{ key, desc string }

	navigation := []binding{
		{"j / k / ↑ / ↓", "move cursor"},
		{"g / home", "jump to top"},
		{"G / end", "jump to bottom"},
		{"ctrl+u / pgup", "half-page up"},
		{"ctrl+d / pgdown", "half-page down"},
		{"tab / shift+tab", "cycle views (Source → Agents → Projects)"},
		{"r", "rescan source and targets"},
		{"? / esc", "close this help"},
		{"q", "quit"},
	}

	actions := []binding{
		{"Enter", "open context palette for item under cursor"},
		{"1 – 9", "trigger nth palette action directly (no palette shown)"},
		{"", "destructive actions always show a confirm prompt"},
	}

	globalCmds := []binding{
		{paletteHintKey(), "open global command palette"},
		{"", "palette contains: sync all, rescan, add/remove target,"},
		{"", "discover agents, add project, init wizard"},
	}

	filters := []binding{
		{"f", "focus kind filter row (Source view)"},
		{"t", "focus target filter row (Source / Agents view)"},
		{"← →", "cycle options in focused filter row"},
		{"f / t", "switch between filter rows when one is focused"},
		{"esc / j / k", "exit filter focus, return to list navigation"},
	}

	overlayKeys := []binding{
		{"esc", "close overlay / cancel"},
		{"y / n", "confirm or cancel a destructive action"},
		{"tab", "move between input fields in forms"},
		{"enter", "submit current field or form"},
	}

	var sb strings.Builder
	writeSection := func(title string, bindings []binding) {
		sb.WriteString(tabActiveStyle.Render(title))
		sb.WriteByte('\n')
		for _, b := range bindings {
			if b.key == "" {
				fmt.Fprintf(&sb, "  %s\n", dimStyle.Render("  "+b.desc))
			} else {
				fmt.Fprintf(&sb, "  %-22s  %s\n", dimStyle.Render(b.key), b.desc)
			}
		}
		sb.WriteByte('\n')
	}

	writeSection("Navigation", navigation)
	writeSection("Item actions", actions)
	writeSection("Global commands", globalCmds)
	writeSection("Filters", filters)
	writeSection("Overlays & forms", overlayKeys)

	return renderOverlayBox(w, strings.TrimRight(sb.String(), "\n"), "Keybindings", 60)
}

// ── confirmOverlay ────────────────────────────────────────────────────────────

type confirmOverlay struct {
	title  string
	detail string
	onYes  func() error
}

func newConfirmOverlay(title, detail string, onYes func() error) *confirmOverlay {
	return &confirmOverlay{title: title, detail: detail, onYes: onYes}
}

func (o *confirmOverlay) Update(msg tea.Msg) (overlayModel, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return o, nil
	}
	switch key.String() {
	case "ctrl+c":
		return nil, tea.Quit
	case "n", "N", "esc":
		return nil, nil
	case "y", "Y", "enter":
		return nil, func() tea.Msg { return cfgReloadMsg{err: o.onYes()} }
	}
	return o, nil
}

func (o *confirmOverlay) View(w int) string {
	content := o.detail + "\n\n" +
		tabActiveStyle.Render("[ y ]") + " confirm  " +
		dimStyle.Render("[ n / esc ]") + " cancel"
	return renderOverlayBox(w, content, o.title, 52)
}

