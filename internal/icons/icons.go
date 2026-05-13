// Package icons renders compact inline agent badges for terminal display.
//
// Two rendering modes:
//
//   - Kitty graphics protocol (Ghostty, Kitty, any terminal announcing
//     "xterm-kitty"): a crisp scaled inline image of the embedded 64×64 PNG
//     logo. Preload() transmits images once; Badge() returns a tiny placement
//     escape per call.
//
//   - Brand glyph fallback: a single Unicode character in the agent's brand
//     colour, centred within the badge area. Used on every other terminal.
//
// Both modes return labels measuring exactly `cols` cells wide via
// runewidth/lipgloss, so callers can keep aligned columns in forms and
// tables.
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

	// Brand-coloured letter badges used as the fallback for terminals without
	// Kitty graphics support. Each is rendered as a 3-cell filled rectangle
	// (` C ` style) so it reads at a glance like a packaging badge.
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

// IsKittySupported reports whether the running terminal can render Kitty
// graphics protocol escape sequences (Ghostty, Kitty, anything announcing
// xterm-kitty in $TERM).
func IsKittySupported() bool {
	switch os.Getenv("TERM_PROGRAM") {
	case "ghostty", "kitty":
		return true
	}
	return os.Getenv("TERM") == "xterm-kitty"
}

// Preload transmits the given agents' logos to the terminal once so later
// Badge() calls only need to reference them by ID. Safe to call repeatedly:
// each agent is sent at most once per process. No-op on terminals without
// Kitty graphics support.
//
// Call this before starting an interactive form or program that draws
// badges. Kitty image cache persists across alt-screen transitions, so it is
// fine to preload while still in the normal screen.
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

// Badge returns a single-line, cols-wide identifier for agent. When Preload
// has been called for agent and the terminal supports Kitty graphics, the
// badge embeds a crisp 2-cell-wide square inline image of the logo plus
// padding to reach cols. Otherwise it returns a brand-coloured 3-cell filled
// "letter chip" (e.g. ` C ` in orange) padded to cols.
//
// cols must be at least 2; values below are clamped up.
func Badge(agent string, cols int) string {
	if cols < 2 {
		cols = 2
	}
	preloadedMu.Lock()
	pre := preloaded[agent]
	preloadedMu.Unlock()
	if pre {
		if id, ok := kittyIDs[agent]; ok {
			// a=p   place a previously transmitted image
			// c=2,r=1 occupy a 2×1 cell rectangle. Terminal cells are
			//        typically 1:2 (W:H), so 2 cells wide × 1 cell tall is
			//        ~square — matches the 64×64 source aspect without the
			//        vertical overflow seen when forcing wider c values.
			// C=1   do not move cursor; the padding spaces below fill the
			//        cell area and let lipgloss/runewidth measure width.
			// q=2   suppress terminal responses
			return fmt.Sprintf("\x1b_Ga=p,i=%d,c=2,r=1,C=1,q=2\x1b\\%s",
				id, strings.Repeat(" ", cols))
		}
	}
	bb, ok := brandBadges[agent]
	if !ok {
		return strings.Repeat(" ", cols)
	}
	// Filled 3-cell chip ` X `: brand colour background, bold white letter.
	chip := fmt.Sprintf("\x1b[48;2;%d;%d;%dm\x1b[38;2;255;255;255m\x1b[1m %s \x1b[0m",
		bb.r, bb.g, bb.b, bb.letter)
	pad := cols - 3
	if pad < 0 {
		pad = 0
	}
	return chip + strings.Repeat(" ", pad)
}

func transmitKitty(pngData []byte, id uint32) {
	encoded := base64.StdEncoding.EncodeToString(pngData)
	const maxChunk = 4096
	for i := 0; i < len(encoded); i += maxChunk {
		end := i + maxChunk
		if end > len(encoded) {
			end = len(encoded)
		}
		chunk := encoded[i:end]
		more := "0"
		if end < len(encoded) {
			more = "1"
		}
		if i == 0 {
			// a=t   transmit only, do not display
			// f=100 source is PNG
			// i=ID, q=2 suppress responses, m=more chunks
			fmt.Fprintf(os.Stdout, "\x1b_Ga=t,f=100,i=%d,q=2,m=%s;%s\x1b\\",
				id, more, chunk)
		} else {
			fmt.Fprintf(os.Stdout, "\x1b_Gm=%s,q=2;%s\x1b\\", more, chunk)
		}
	}
}
