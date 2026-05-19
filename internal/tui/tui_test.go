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
