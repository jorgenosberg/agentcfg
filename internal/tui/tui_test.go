package tui

import (
	"testing"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/source"
)

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
