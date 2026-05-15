package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jorgenosberg/agentcfg/internal/catalog"
	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/icons"
	"github.com/jorgenosberg/agentcfg/internal/source"
	"github.com/jorgenosberg/agentcfg/internal/sync"
	"github.com/jorgenosberg/agentcfg/internal/wizard"
)

// cfgReloadMsg is dispatched when any management operation completes.
type cfgReloadMsg struct{ err error }

// overlayModel is the in-TUI overlay interface. Returning nil from Update dismisses the overlay.
type overlayModel interface {
	Update(tea.Msg) (overlayModel, tea.Cmd)
	View(w, h int) string
}

func renderOverlayBox(w, h int, content, title string, boxW int) string {
	if boxW > w-4 {
		boxW = w - 4
	}
	if boxW < 20 {
		boxW = 20
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2).
		Width(boxW).
		Render(titleStyle.Render(title) + "\n\n" + content)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
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

func (o *helpOverlay) View(w, h int) string {
	type binding struct{ key, desc string }
	global := []binding{
		{"j / k / ↑↓", "move cursor"},
		{"g / home", "go to top"},
		{"G / end", "go to bottom"},
		{"ctrl+u / pgup", "half-page up"},
		{"ctrl+d / pgdown", "half-page down"},
		{"1 / 2 / tab", "switch view"},
		{"r", "rescan source"},
		{"I", "run init wizard"},
		{"D", "discover agents"},
		{"? / esc", "close this help"},
		{"q", "quit"},
	}
	sourceOnly := []binding{
		{"enter / i", "install selected item"},
		{"x", "uninstall selected item"},
		{"S", "sync all (install absent + update drifted)"},
		{"f", "cycle filter (all / skills / hooks / context)"},
		{"n", "add target"},
		{"d", "remove target"},
	}
	projectsOnly := []binding{
		{"n", "add project"},
		{"d", "remove project"},
	}

	var sb strings.Builder
	writeSection := func(title string, bindings []binding) {
		sb.WriteString(tabActiveStyle.Render(title))
		sb.WriteByte('\n')
		for _, b := range bindings {
			fmt.Fprintf(&sb, "  %-22s  %s\n", dimStyle.Render(b.key), b.desc)
		}
		sb.WriteByte('\n')
	}
	writeSection("Global", global)
	writeSection("Source view", sourceOnly)
	writeSection("Projects view", projectsOnly)

	return renderOverlayBox(w, h, strings.TrimRight(sb.String(), "\n"), "Keybindings", 56)
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

func (o *confirmOverlay) View(w, h int) string {
	content := o.detail + "\n\n" +
		tabActiveStyle.Render("[ y ]")+" confirm  "+
		dimStyle.Render("[ n / esc ]")+" cancel"
	return renderOverlayBox(w, h, content, o.title, 52)
}

// ── addTargetOverlay ──────────────────────────────────────────────────────────

type addTargetOverlay struct {
	cfgPath string
	cfg     config.Config
	name    textinput.Model
	path    textinput.Model
	focused int
	errMsg  string
}

func newAddTargetOverlay(cfgPath string, cfg config.Config) (*addTargetOverlay, tea.Cmd) {
	name := textinput.New()
	name.Placeholder = `e.g. "claude"`
	name.Prompt = "Name: "
	name.PromptStyle = dimStyle
	name.Width = 38
	path := textinput.New()
	path.Placeholder = "absolute path to agent config dir"
	path.Prompt = "Path: "
	path.PromptStyle = dimStyle
	path.Width = 38
	cmd := name.Focus()
	return &addTargetOverlay{cfgPath: cfgPath, cfg: cfg, name: name, path: path}, cmd
}

func (o *addTargetOverlay) inputs() []*textinput.Model {
	return []*textinput.Model{&o.name, &o.path}
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
			cfg.Targets = append(cfg.Targets, config.Target{Name: name, Path: abs})
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

func (o *addTargetOverlay) View(w, h int) string {
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
	return renderOverlayBox(w, h, sb.String(), "Add target", 54)
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

func (o *addProjectOverlay) View(w, h int) string {
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
	return renderOverlayBox(w, h, sb.String(), "Add project", 54)
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
	names := make([]string, len(candidates))
	for i, t := range candidates {
		names[i] = t.Name
	}
	icons.Preload(names)
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

func (o *discoverOverlay) View(w, h int) string {
	if o.empty {
		content := dimStyle.Render("No new agent directories found on disk.") +
			"\n\n" + dimStyle.Render("esc close")
		return renderOverlayBox(w, h, content, "Discover agents", 52)
	}
	selCount := len(o.ms.selectedItems())
	header := dimStyle.Render(fmt.Sprintf("%d / %d selected", selCount, len(o.ms.items)))
	content := header + "\n\n" + o.ms.View() + "\n\n" +
		dimStyle.Render("j/k move · space toggle · ctrl+a all/none · enter confirm · esc cancel")
	return renderOverlayBox(w, h, content, "Discover agents", max(52, w*3/5))
}

// ── initWizardOverlay ─────────────────────────────────────────────────────────

type wizardStep int

const (
	wizardStepSource  wizardStep = iota
	wizardStepTargets            // multi-select discovered agents
	wizardStepItems              // multi-select items to import (may be skipped)
	wizardStepProject            // confirm add project
	wizardStepProjDetails        // name+path inputs (only if addProject=true)
)

type wizardItem struct {
	target config.Target
	item   source.Item
	key    string
}

type initWizardOverlay struct {
	cfgPath string
	step    wizardStep

	// step: source
	srcInput textinput.Model

	// step: targets
	targetCandidates []config.Target
	targetMS         multiSelect

	// step: items
	wizardItems []wizardItem
	itemMS      multiSelect

	// step: project confirm
	addProject bool

	// step: project details
	projName   textinput.Model
	projPath   textinput.Model
	projFocus  int

	errMsg string
}

func newInitWizardOverlay(cfgPath string) (*initWizardOverlay, tea.Cmd) {
	src := ""
	if s, err := config.DefaultSource(); err == nil {
		src = s
	}
	inp := textinput.New()
	inp.Placeholder = "e.g. ~/.agentcfg/source"
	inp.Prompt = "Source path: "
	inp.PromptStyle = dimStyle
	inp.Width = 44
	inp.SetValue(src)
	cmd := inp.Focus()
	return &initWizardOverlay{cfgPath: cfgPath, srcInput: inp}, cmd
}

func (o *initWizardOverlay) advanceToTargets() {
	found := catalog.Discover()
	o.targetCandidates = found
	names := make([]string, len(found))
	for i, t := range found {
		names[i] = t.Name
	}
	if len(names) > 0 {
		icons.Preload(names)
	}
	items := make([]msItem, len(found))
	for i, t := range found {
		badge := icons.TextBadge(t.Name, 3)
		items[i] = msItem{
			label: fmt.Sprintf("%s  %-10s  %s", badge, t.Name, t.Path),
			value: t.Name,
		}
	}
	o.targetMS = newMultiSelect(items, 10)
	o.step = wizardStepTargets
}

func (o *initWizardOverlay) advanceToItems() {
	type rawCand struct {
		target config.Target
		item   source.Item
		hash   string
	}
	var raw []rawCand
	for i, it := range o.targetMS.items {
		if !it.selected {
			continue
		}
		t := o.targetCandidates[i]
		items, err := source.ScanWith(t.Path, t.Subdirs)
		if err != nil {
			continue
		}
		for _, item := range items {
			h, err := wizard.ContentHash(item.Path)
			if err != nil {
				h = t.Name + ":" + item.Path
			}
			raw = append(raw, rawCand{target: t, item: item, hash: h})
		}
	}

	type hashGroup struct {
		entries []rawCand
		hashes  map[string]bool
	}
	groups := map[string]*hashGroup{}
	var order []string
	for _, rc := range raw {
		k := rc.item.Kind + "/" + rc.item.Name
		if _, ok := groups[k]; !ok {
			groups[k] = &hashGroup{hashes: map[string]bool{}}
			order = append(order, k)
		}
		g := groups[k]
		if !g.hashes[rc.hash] {
			g.hashes[rc.hash] = true
			g.entries = append(g.entries, rc)
		}
	}

	o.wizardItems = nil
	msItems := []msItem{}
	for _, k := range order {
		g := groups[k]
		multi := len(g.hashes) > 1
		for _, rc := range g.entries {
			key := rc.item.Kind + "/" + rc.item.Name
			badge := icons.TextBadge(rc.target.Name, 3)
			label := fmt.Sprintf("%s  %-8s  %s", badge, rc.item.Kind, rc.item.Name)
			if multi {
				key = rc.target.Name + "/" + rc.item.Kind + "/" + rc.item.Name
				label = fmt.Sprintf("%s  %-8s  %-24s  [%s]", badge, rc.item.Kind, rc.item.Name, rc.target.Name)
			}
			o.wizardItems = append(o.wizardItems, wizardItem{target: rc.target, item: rc.item, key: key})
			msItems = append(msItems, msItem{label: label, value: key})
		}
	}
	o.itemMS = newMultiSelect(msItems, 10)

	if len(o.wizardItems) == 0 {
		o.step = wizardStepProject
	} else {
		o.step = wizardStepItems
	}
}

func (o *initWizardOverlay) advanceToProjDetails() {
	n := textinput.New()
	n.Placeholder = `e.g. "myapp"`
	n.Prompt = "Name: "
	n.PromptStyle = dimStyle
	n.Width = 38
	p := textinput.New()
	p.Placeholder = "path to repository root"
	p.Prompt = "Path: "
	p.PromptStyle = dimStyle
	p.Width = 38
	n.Focus()
	o.projName = n
	o.projPath = p
	o.projFocus = 0
	o.step = wizardStepProjDetails
}

func (o *initWizardOverlay) apply() error {
	src := strings.TrimSpace(o.srcInput.Value())
	if src == "" {
		return fmt.Errorf("source path is required")
	}
	for _, sub := range source.DefaultSubdirs {
		if sub == "" {
			continue
		}
		if err := os.MkdirAll(filepath.Join(src, sub), 0o755); err != nil {
			return fmt.Errorf("create source subdir: %w", err)
		}
	}

	cfg := config.Default(src)
	for i, it := range o.targetMS.items {
		if it.selected {
			cfg.Targets = append(cfg.Targets, o.targetCandidates[i])
		}
	}
	if o.addProject {
		projName := strings.TrimSpace(o.projName.Value())
		projPath := strings.TrimSpace(o.projPath.Value())
		if projName != "" && projPath != "" {
			abs, err := filepath.Abs(projPath)
			if err != nil {
				abs = projPath
			}
			cfg.Projects = append(cfg.Projects, config.Project{Name: projName, Path: abs})
		}
	}
	if err := config.Save(o.cfgPath, cfg); err != nil {
		return err
	}

	// Import selected items
	for i, wi := range o.wizardItems {
		if !o.itemMS.items[i].selected {
			continue
		}
		destSub := source.DefaultSubdirs[wi.item.Kind]
		destDir := filepath.Join(src, destSub)
		dest := filepath.Join(destDir, wi.item.Name)
		if _, err := os.Lstat(dest); err == nil {
			continue
		}
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			continue
		}
		sync.CopyAny(wi.item.Path, dest) //nolint:errcheck
	}
	return nil
}

func (o *initWizardOverlay) Update(msg tea.Msg) (overlayModel, tea.Cmd) {
	// Forward non-key messages to focused textinputs for blink
	key, isKey := msg.(tea.KeyMsg)
	if !isKey {
		switch o.step {
		case wizardStepSource:
			var cmd tea.Cmd
			o.srcInput, cmd = o.srcInput.Update(msg)
			return o, cmd
		case wizardStepProjDetails:
			var cmd tea.Cmd
			if o.projFocus == 0 {
				o.projName, cmd = o.projName.Update(msg)
			} else {
				o.projPath, cmd = o.projPath.Update(msg)
			}
			return o, cmd
		}
		return o, nil
	}

	if key.String() == "ctrl+c" {
		return nil, tea.Quit
	}
	if key.String() == "esc" {
		return nil, nil
	}

	o.errMsg = ""

	switch o.step {
	case wizardStepSource:
		if key.String() == "enter" {
			if strings.TrimSpace(o.srcInput.Value()) == "" {
				o.errMsg = "source path is required"
				return o, nil
			}
			o.srcInput.Blur()
			o.advanceToTargets()
			return o, nil
		}
		var cmd tea.Cmd
		o.srcInput, cmd = o.srcInput.Update(key)
		return o, cmd

	case wizardStepTargets:
		switch key.String() {
		case "enter":
			o.advanceToItems()
		default:
			o.targetMS.handleKey(key.String())
		}

	case wizardStepItems:
		switch key.String() {
		case "enter":
			o.step = wizardStepProject
		default:
			o.itemMS.handleKey(key.String())
		}

	case wizardStepProject:
		switch key.String() {
		case "left", "right", "h", "l", "tab", " ":
			o.addProject = !o.addProject
		case "y", "Y":
			o.addProject = true
			o.advanceToProjDetails()
			return o, o.projName.Focus()
		case "n", "N":
			o.addProject = false
			return nil, func() tea.Msg { return cfgReloadMsg{err: o.apply()} }
		case "enter":
			if o.addProject {
				o.advanceToProjDetails()
				return o, o.projName.Focus()
			}
			return nil, func() tea.Msg { return cfgReloadMsg{err: o.apply()} }
		}

	case wizardStepProjDetails:
		switch key.String() {
		case "tab", "down":
			if o.projFocus == 0 {
				o.projName.Blur()
				o.projFocus = 1
				return o, o.projPath.Focus()
			}
		case "shift+tab", "up":
			if o.projFocus == 1 {
				o.projPath.Blur()
				o.projFocus = 0
				return o, o.projName.Focus()
			}
		case "enter":
			if o.projFocus == 0 {
				o.projName.Blur()
				o.projFocus = 1
				return o, o.projPath.Focus()
			}
			return nil, func() tea.Msg { return cfgReloadMsg{err: o.apply()} }
		default:
			var cmd tea.Cmd
			if o.projFocus == 0 {
				o.projName, cmd = o.projName.Update(key)
			} else {
				o.projPath, cmd = o.projPath.Update(key)
			}
			return o, cmd
		}
	}

	return o, nil
}

func (o *initWizardOverlay) stepLabel() string {
	labels := []string{"source", "targets", "import", "project"}
	styledLabels := make([]string, len(labels))
	currentIdx := int(o.step)
	if o.step == wizardStepProjDetails {
		currentIdx = int(wizardStepProject)
	}
	for i, l := range labels {
		if i == currentIdx {
			styledLabels[i] = tabActiveStyle.Render(l)
		} else {
			styledLabels[i] = dimStyle.Render(l)
		}
	}
	return strings.Join(styledLabels, dimStyle.Render(" → "))
}

func (o *initWizardOverlay) View(w, h int) string {
	var sb strings.Builder
	sb.WriteString(o.stepLabel())
	sb.WriteString("\n\n")

	switch o.step {
	case wizardStepSource:
		sb.WriteString(dimStyle.Render("Directory agentcfg syncs from into all configured targets."))
		sb.WriteString("\n\n  ")
		sb.WriteString(o.srcInput.View())
		sb.WriteString("\n\n")
		if o.errMsg != "" {
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render("  "+o.errMsg) + "\n\n")
		}
		sb.WriteString(dimStyle.Render("enter next · esc cancel"))

	case wizardStepTargets:
		selCount := len(o.targetMS.selectedItems())
		sb.WriteString(dimStyle.Render(fmt.Sprintf("%d / %d selected  —  agent directories to sync into", selCount, len(o.targetMS.items))))
		sb.WriteString("\n\n")
		if len(o.targetMS.items) == 0 {
			sb.WriteString(dimStyle.Render("  no agent directories found on disk\n"))
		} else {
			sb.WriteString(o.targetMS.View())
		}
		sb.WriteString("\n\n")
		sb.WriteString(dimStyle.Render("j/k move · space toggle · ctrl+a all/none · enter next · esc cancel"))

	case wizardStepItems:
		selCount := len(o.itemMS.selectedItems())
		sb.WriteString(dimStyle.Render(fmt.Sprintf("%d / %d selected  —  items to copy into your source tree", selCount, len(o.itemMS.items))))
		sb.WriteString("\n\n")
		sb.WriteString(o.itemMS.View())
		sb.WriteString("\n\n")
		sb.WriteString(dimStyle.Render("j/k move · space toggle · ctrl+a all/none · enter next · esc cancel"))

	case wizardStepProject:
		sb.WriteString(dimStyle.Render("Scan a repository for local agent config files (CLAUDE.md, .cursorrules, etc.)."))
		sb.WriteString("\n\n  Add a project folder?  ")
		if o.addProject {
			sb.WriteString(tabActiveStyle.Render("[ Yes ]") + "  " + dimStyle.Render("[ No  ]"))
		} else {
			sb.WriteString(dimStyle.Render("[ Yes ]") + "  " + tabActiveStyle.Render("[ No  ]"))
		}
		sb.WriteString("\n\n")
		sb.WriteString(dimStyle.Render("←/→/space switch · y/n choose · enter confirm · esc cancel"))

	case wizardStepProjDetails:
		sb.WriteString(dimStyle.Render("Repository root to scan for in-repo agent config."))
		sb.WriteString("\n\n")
		for i, inp := range []textinput.Model{o.projName, o.projPath} {
			if i == o.projFocus {
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
		sb.WriteString(dimStyle.Render("tab next field · enter finish · esc cancel"))
	}

	return renderOverlayBox(w, h, sb.String(), "Init wizard", max(56, w*3/5))
}
