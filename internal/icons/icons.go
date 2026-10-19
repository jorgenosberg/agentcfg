// Package icons renders compact agent badges for terminal display: a colored
// letter chip (` C `) that measures exactly `cols` cells wide, so callers can
// keep aligned columns.
package icons

import (
	"fmt"
	"strings"
)

var brandBadges = map[string]struct {
	letter     string
	r, g, b    uint8
	tr, tg, tb uint8 // text color (defaults to white 255,255,255 if all zero)
}{
	"claude":   {"C", 0xDA, 0x77, 0x56, 0, 0, 0},          // #DA7756 Anthropic terracotta
	"codex":    {"X", 0x10, 0xA3, 0x7F, 0, 0, 0},          // #10A37F OpenAI teal
	"copilot":  {"P", 0x85, 0x34, 0xF3, 0, 0, 0},          // #8534F3 GitHub Copilot purple
	"gemini":   {"G", 0x47, 0x96, 0xE3, 0, 0, 0},          // #4796E3 Gemini blue
	"opencode": {"O", 0xE5, 0xE7, 0xEB, 0x11, 0x11, 0x11}, // #E5E7EB light gray bg, #111 text
	"agents":   {"A", 0x64, 0x74, 0x8B, 0, 0, 0},          // #64748B neutral slate
}

// BrandColor returns the hex brand background color for agent (e.g. "#DA7756"), or "" if unknown.
func BrandColor(agent string) string {
	bb, ok := brandBadges[agent]
	if !ok {
		return ""
	}
	return fmt.Sprintf("#%02X%02X%02X", bb.r, bb.g, bb.b)
}

// TextBadge returns a cols-wide colored letter chip for agent, or blanks if
// the agent is unknown.
func TextBadge(agent string, cols int) string {
	if cols < 2 {
		cols = 2
	}
	bb, ok := brandBadges[agent]
	if !ok {
		return strings.Repeat(" ", cols)
	}
	tr, tg, tb := bb.tr, bb.tg, bb.tb
	if tr == 0 && tg == 0 && tb == 0 {
		tr, tg, tb = 255, 255, 255
	}
	chip := fmt.Sprintf("\x1b[48;2;%d;%d;%dm\x1b[38;2;%d;%d;%dm\x1b[1m %s \x1b[0m",
		bb.r, bb.g, bb.b, tr, tg, tb, bb.letter)
	pad := max(0, cols-3)
	return chip + strings.Repeat(" ", pad)
}
