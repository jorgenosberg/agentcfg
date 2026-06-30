package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/jorgenosberg/agentcfg/internal/icons"
)

func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		if m := int(d.Minutes()); m == 1 {
			return "1 minute ago"
		} else {
			return fmt.Sprintf("%d minutes ago", m)
		}
	case d < 24*time.Hour:
		if h := int(d.Hours()); h == 1 {
			return "1 hour ago"
		} else {
			return fmt.Sprintf("%d hours ago", h)
		}
	case d < 7*24*time.Hour:
		if days := int(d.Hours() / 24); days == 1 {
			return "1 day ago"
		} else {
			return fmt.Sprintf("%d days ago", days)
		}
	case d < 365*24*time.Hour:
		return t.Format("Jan 2")
	default:
		return t.Format("Jan 2, 2006")
	}
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

// withBg applies the selected-row background to s, re-applying the background
// escape after every SGR-0 reset so inline-styled tokens (brand badges, status
// symbols) don't break the full-row highlight.
func withBg(s string) string {
	const (
		bg   = "\x1b[48;5;78m\x1b[30m"
		sgr0 = "\x1b[0m"
	)
	return bg + strings.ReplaceAll(s, sgr0, sgr0+bg) + sgr0
}

func agentNameStyled(agent string, fieldW int) string {
	padded := fmt.Sprintf("%-*s", fieldW, agent)
	hex := icons.BrandColor(agent)
	if hex == "" {
		return padded
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Bold(true).Render(padded)
}
