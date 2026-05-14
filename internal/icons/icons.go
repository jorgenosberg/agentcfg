// Package icons renders compact inline agent badges for terminal display.
//
// Two rendering modes:
//   - Kitty graphics protocol (Ghostty, Kitty, xterm-kitty): inline PNG logo.
//     Preload() transmits images once; Badge() emits a placement escape per call.
//   - Text fallback: a colored letter chip (` C `) for every other terminal.
//
// Both modes return labels measuring exactly `cols` cells wide so callers can
// keep aligned columns.
package icons

import (
	"embed"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"sync"
)

//go:embed data/*.png
var data embed.FS

var (
	rawLogos = map[string][]byte{}

	kittyIDs = map[string]uint32{
		"claude":   1001,
		"codex":    1002,
		"copilot":  1003,
		"gemini":   1004,
		"opencode": 1005,
		"agents":   1006,
	}

	brandBadges = map[string]struct {
		letter  string
		r, g, b uint8
	}{
		"claude":   {"C", 0xD9, 0x77, 0x57}, // Anthropic peach
		"codex":    {"X", 0x6E, 0x9B, 0xFF}, // pale blue
		"copilot":  {"P", 0x8B, 0x5C, 0xF6}, // Copilot purple
		"gemini":   {"G", 0x4F, 0x88, 0xFF}, // Google blue
		"opencode": {"O", 0x9B, 0x5C, 0xFF}, // violet
		"agents":   {"A", 0x10, 0xA3, 0x7C}, // OpenAI teal
	}

	preloaded   = map[string]bool{}
	preloadedMu sync.Mutex
)

func init() {
	names := []string{"claude", "copilot", "codex", "gemini", "opencode", "agents"}
	for _, n := range names {
		if b, err := data.ReadFile("data/" + n + ".png"); err == nil {
			rawLogos[n] = b
		}
	}
}

// Has reports whether an embedded logo exists for agent.
func Has(agent string) bool {
	_, ok := rawLogos[agent]
	return ok
}

// IsKittySupported reports whether the running terminal supports the Kitty
// graphics protocol (Ghostty, Kitty, anything announcing xterm-kitty).
func IsKittySupported() bool {
	switch os.Getenv("TERM_PROGRAM") {
	case "ghostty", "kitty":
		return true
	}
	return os.Getenv("TERM") == "xterm-kitty"
}

// Preload transmits agent logos to the terminal once so Badge() calls only
// need to reference them by ID. Safe to call repeatedly; each agent is sent
// at most once per process. No-op on terminals without Kitty support.
//
// Call before starting an interactive program that draws badges. Kitty image
// cache persists across alt-screen transitions, so preloading in the normal
// screen is fine.
func Preload(agents []string) {
	if !IsKittySupported() {
		return
	}
	preloadedMu.Lock()
	defer preloadedMu.Unlock()
	for _, a := range agents {
		if preloaded[a] {
			continue
		}
		raw, ok := rawLogos[a]
		if !ok {
			continue
		}
		id, ok := kittyIDs[a]
		if !ok {
			continue
		}
		transmitKitty(raw, id)
		preloaded[a] = true
	}
}

// Badge returns a cols-wide identifier for agent. Uses an inline Kitty image
// when Preload has been called and the terminal supports it; otherwise falls
// back to a colored letter chip (same as TextBadge).
//
// Kitty placements accumulate across re-renders — only use Badge where the
// image position is stable between frames. Use TextBadge for overlays and
// scrollable widgets.
func Badge(agent string, cols int) string {
	if cols < 2 {
		cols = 2
	}
	preloadedMu.Lock()
	pre := preloaded[agent]
	preloadedMu.Unlock()
	if pre {
		if id, ok := kittyIDs[agent]; ok {
			// a=p: place preloaded image; c=2,r=1: 2×1 cell area (~square for
			// 64×64 source); C=1: don't move cursor; trailing spaces advance it.
			return fmt.Sprintf("\x1b_Ga=p,i=%d,c=2,r=1,C=1,q=2\x1b\\%s",
				id, strings.Repeat(" ", cols))
		}
	}
	return TextBadge(agent, cols)
}

// TextBadge returns a cols-wide colored letter chip regardless of terminal
// capabilities. Use in overlays, scrollable lists, and huh forms where Kitty
// placements can't be cleaned up between re-renders.
func TextBadge(agent string, cols int) string {
	if cols < 2 {
		cols = 2
	}
	bb, ok := brandBadges[agent]
	if !ok {
		return strings.Repeat(" ", cols)
	}
	chip := fmt.Sprintf("\x1b[48;2;%d;%d;%dm\x1b[38;2;255;255;255m\x1b[1m %s \x1b[0m",
		bb.r, bb.g, bb.b, bb.letter)
	pad := max(0, cols-3)
	return chip + strings.Repeat(" ", pad)
}

func transmitKitty(pngData []byte, id uint32) {
	encoded := base64.StdEncoding.EncodeToString(pngData)
	const maxChunk = 4096
	for i := 0; i < len(encoded); i += maxChunk {
		end := min(i+maxChunk, len(encoded))
		chunk := encoded[i:end]
		more := "0"
		if end < len(encoded) {
			more = "1"
		}
		if i == 0 {
			fmt.Fprintf(os.Stdout, "\x1b_Ga=t,f=100,i=%d,q=2,m=%s;%s\x1b\\",
				id, more, chunk)
		} else {
			fmt.Fprintf(os.Stdout, "\x1b_Gm=%s,q=2;%s\x1b\\", more, chunk)
		}
	}
}
