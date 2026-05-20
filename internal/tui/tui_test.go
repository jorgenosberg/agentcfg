package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/source"
)

// TestMain forces CLICOLOR_FORCE so lipgloss renders ANSI codes in non-TTY test
// environments. Tests that rely on color output (e.g. dimBackground) require this.
func TestMain(m *testing.M) {
	os.Setenv("CLICOLOR_FORCE", "1")
	os.Exit(m.Run())
}

func TestNextKind(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", source.KindSkill},
		{source.KindSkill, source.KindHook},
		{source.KindHook, source.KindContext},
		{source.KindContext, ""},
	}
	for _, c := range cases {
		if got := nextKind(c.in); got != c.want {
			t.Errorf("nextKind(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPrevKind(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", source.KindContext},
		{source.KindContext, source.KindHook},
		{source.KindHook, source.KindSkill},
		{source.KindSkill, ""},
	}
	for _, c := range cases {
		if got := prevKind(c.in); got != c.want {
			t.Errorf("prevKind(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNextTarget(t *testing.T) {
	targets := []config.Target{{Name: "claude"}, {Name: "cursor"}, {Name: "zed"}}
	cases := []struct{ in, want string }{
		{"", "claude"},
		{"claude", "cursor"},
		{"cursor", "zed"},
		{"zed", ""},
	}
	for _, c := range cases {
		if got := nextTarget(c.in, targets); got != c.want {
			t.Errorf("nextTarget(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPrevTarget(t *testing.T) {
	targets := []config.Target{{Name: "claude"}, {Name: "cursor"}, {Name: "zed"}}
	cases := []struct{ in, want string }{
		{"", "zed"},
		{"claude", ""},
		{"cursor", "claude"},
		{"zed", "cursor"},
	}
	for _, c := range cases {
		if got := prevTarget(c.in, targets); got != c.want {
			t.Errorf("prevTarget(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPaletteWidgetMoveDown(t *testing.T) {
	pw := paletteWidget{actions: []paletteAction{
		{label: "a"}, {label: "b"}, {label: "c"},
	}}
	pw.moveDown()
	if pw.cursor != 1 {
		t.Fatalf("expected 1, got %d", pw.cursor)
	}
	pw.moveDown()
	pw.moveDown() // should clamp
	if pw.cursor != 2 {
		t.Fatalf("expected 2 (clamped), got %d", pw.cursor)
	}
}

func TestPaletteWidgetMoveUp(t *testing.T) {
	pw := paletteWidget{actions: []paletteAction{
		{label: "a"}, {label: "b"},
	}, cursor: 1}
	pw.moveUp()
	if pw.cursor != 0 {
		t.Fatalf("expected 0, got %d", pw.cursor)
	}
	pw.moveUp() // should clamp
	if pw.cursor != 0 {
		t.Fatalf("expected 0 (clamped), got %d", pw.cursor)
	}
}

func TestPaletteWidgetActionAt(t *testing.T) {
	pw := paletteWidget{actions: []paletteAction{
		{label: "first"}, {label: "second"}, {label: "third"},
	}}
	if a := pw.actionAt(1); a == nil || a.label != "first" {
		t.Fatalf("actionAt(1) = %v", a)
	}
	if a := pw.actionAt(3); a == nil || a.label != "third" {
		t.Fatalf("actionAt(3) = %v", a)
	}
	if a := pw.actionAt(0); a != nil {
		t.Fatalf("actionAt(0) should be nil, got %v", a)
	}
	if a := pw.actionAt(4); a != nil {
		t.Fatalf("actionAt(4) should be nil, got %v", a)
	}
}

func TestPaletteWidgetSelected(t *testing.T) {
	pw := paletteWidget{actions: []paletteAction{{label: "only"}}}
	if a := pw.selected(); a == nil || a.label != "only" {
		t.Fatalf("unexpected: %v", a)
	}
	empty := paletteWidget{}
	if a := empty.selected(); a != nil {
		t.Fatalf("expected nil for empty widget, got %v", a)
	}
}

func TestDimBackgroundStripsColors(t *testing.T) {
	styled := lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true).Render("hello world")
	result := dimBackground(styled)
	if got := ansi.Strip(result); got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
	if result == styled {
		t.Error("dimBackground should change styling")
	}
}

func TestDimBackgroundPreservesLineCount(t *testing.T) {
	input := "first\nsecond\nthird"
	result := dimBackground(input)
	resultLines := strings.Split(result, "\n")
	inputLines := strings.Split(input, "\n")
	if len(resultLines) != len(inputLines) {
		t.Fatalf("got %d lines, want %d", len(resultLines), len(inputLines))
	}
	for i, line := range resultLines {
		if got := ansi.Strip(line); got != inputLines[i] {
			t.Errorf("line %d: got %q, want %q", i, got, inputLines[i])
		}
	}
}

func TestPasteOverlayBasic(t *testing.T) {
	bg := "1234567890\n1234567890\n1234567890"
	popup := "AB\nCD"
	result := pasteOverlay(bg, popup, 2, 1)
	lines := strings.Split(result, "\n")
	if got := ansi.Strip(lines[0]); got != "1234567890" {
		t.Errorf("line 0: got %q, want %q", got, "1234567890")
	}
	if got := ansi.Strip(lines[1]); got != "12AB567890" {
		t.Errorf("line 1: got %q, want %q", got, "12AB567890")
	}
	if got := ansi.Strip(lines[2]); got != "12CD567890" {
		t.Errorf("line 2: got %q, want %q", got, "12CD567890")
	}
}

func TestPasteOverlayAtOrigin(t *testing.T) {
	bg := "AAAA\nAAAA"
	popup := "BB"
	result := pasteOverlay(bg, popup, 0, 0)
	lines := strings.Split(result, "\n")
	if got := ansi.Strip(lines[0]); got != "BBAA" {
		t.Errorf("line 0: got %q, want %q", got, "BBAA")
	}
	if got := ansi.Strip(lines[1]); got != "AAAA" {
		t.Errorf("line 1 should be unchanged: got %q, want %q", got, "AAAA")
	}
}

func TestPasteOverlayPadsShortLine(t *testing.T) {
	bg := "AB\nXX"
	popup := "12345"
	result := pasteOverlay(bg, popup, 4, 0)
	lines := strings.Split(result, "\n")
	// "AB" padded to col 4, then popup inserted
	if got := ansi.Strip(lines[0]); got != "AB  12345" {
		t.Errorf("line 0: got %q, want %q", got, "AB  12345")
	}
}

func TestPasteOverlaySkipsOutOfBoundsRows(t *testing.T) {
	bg := "line0\nline1"
	popup := "X\nY\nZ"
	// y=1 means popup rows land at bg lines 1, 2, 3 — line 2 and 3 don't exist
	result := pasteOverlay(bg, popup, 0, 1)
	lines := strings.Split(result, "\n")
	if len(lines) != 2 {
		t.Errorf("background line count should not change: got %d", len(lines))
	}
	if got := ansi.Strip(lines[0]); got != "line0" {
		t.Errorf("line 0 should be unchanged: got %q", got)
	}
	if got := ansi.Strip(lines[1]); got != "Xine1" {
		t.Errorf("line 1: got %q, want %q", got, "Xine1")
	}
}

func TestRenderMainViewFooterSeparator(t *testing.T) {
	// Minimal model with explicit dimensions.
	m := newModel("", config.Config{}, nil, nil)
	m.width = 80
	m.height = 24
	out := m.renderMainView()
	lines := strings.Split(out, "\n")
	if len(lines) < 2 {
		t.Fatal("expected at least 2 lines in renderMainView output")
	}
	// renderFooter has no trailing newline, so it becomes lines[last].
	// The separator (with its trailing \n) is lines[last-1].
	sep := ansi.Strip(lines[len(lines)-2])
	if !strings.HasPrefix(sep, "─") {
		t.Errorf("expected separator line before footer, got: %q", sep)
	}
}
