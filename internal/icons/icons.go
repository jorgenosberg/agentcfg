// Package icons provides agent logo rendering for terminal display.
// Logos are embedded as 64×64 RGBA PNGs. Two rendering backends are provided:
//
//   - Kitty graphics protocol (IsKittySupported): sends the raw PNG to the
//     terminal which scales it to the requested cell dimensions. Used by
//     Ghostty, the Kitty terminal, and any terminal with Kitty protocol support.
//     Produces sharp, high-quality output at any size.
//
//   - Unicode half-block art (Gallery / Render / RenderLine): renders logos as
//     ▀ characters with true-colour ANSI foreground/background pairs, using
//     area-averaging for smooth downscaling. Fallback for all other terminals.
package icons

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strconv"
	"strings"
	"sync"

	charmbterm "github.com/charmbracelet/x/term"
)

//go:embed data/*.png
var data embed.FS

var (
	rawLogos   map[string][]byte
	imgCache   = map[string]image.Image{}
	imgCacheMu sync.RWMutex
)

func init() {
	names := []string{"claude", "copilot", "codex", "gemini", "opencode", "agents"}
	rawLogos = make(map[string][]byte, len(names))
	for _, n := range names {
		b, err := data.ReadFile("data/" + n + ".png")
		if err == nil {
			rawLogos[n] = b
		}
	}
}

func decodedImage(agent string) (image.Image, bool) {
	imgCacheMu.RLock()
	img, ok := imgCache[agent]
	imgCacheMu.RUnlock()
	if ok {
		return img, true
	}
	b, exists := rawLogos[agent]
	if !exists {
		return nil, false
	}
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, false
	}
	imgCacheMu.Lock()
	imgCache[agent] = img
	imgCacheMu.Unlock()
	return img, true
}

// Has reports whether an embedded logo exists for agent.
func Has(agent string) bool {
	_, ok := rawLogos[agent]
	return ok
}

// ── Kitty graphics protocol ───────────────────────────────────────────────────

// IsKittySupported reports whether the running terminal supports the Kitty
// graphics protocol. Ghostty and the Kitty terminal both qualify.
func IsKittySupported() bool {
	switch os.Getenv("TERM_PROGRAM") {
	case "ghostty", "kitty":
		return true
	}
	return os.Getenv("TERM") == "xterm-kitty"
}

// GalleryKitty renders agent logos side-by-side as a single Kitty graphics
// protocol image. Each icon occupies iconCols terminal columns; iconRows sets
// the display height. For square icons on typical 2:1 (height:width) fonts,
// use iconRows = iconCols / 2 (e.g., cols=10, rows=5).
//
// Returns an empty string when no logos are found.
func GalleryKitty(agents []string, iconCols, iconRows int) string {
	type entry struct {
		name string
		img  image.Image
	}
	var items []entry
	for _, a := range agents {
		if img, ok := decodedImage(a); ok {
			items = append(items, entry{a, img})
		}
	}
	if len(items) == 0 {
		return ""
	}

	n := len(items)
	const srcPx = 64    // source image dimensions (64×64)
	const gapCols = 1   // terminal column gap between icons
	// Keep gap pixels proportional to column gap so terminal scaling is exact.
	gapPx := srcPx * gapCols / iconCols
	totalW := n*srcPx + (n-1)*gapPx

	// Compose logos into a single horizontal strip.
	composed := image.NewRGBA(image.Rect(0, 0, totalW, srcPx))
	for i, it := range items {
		x0 := i * (srcPx + gapPx)
		b := it.img.Bounds()
		sw, sh := b.Max.X-b.Min.X, b.Max.Y-b.Min.Y
		for y := 0; y < srcPx; y++ {
			for x := 0; x < srcPx; x++ {
				sx := b.Min.X + x*sw/srcPx
				sy := b.Min.Y + y*sh/srcPx
				composed.Set(x0+x, y, it.img.At(sx, sy))
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, composed); err != nil {
		return ""
	}

	totalCols := n*iconCols + (n-1)*gapCols
	var sb strings.Builder
	sb.WriteString(kittyEncode(buf.Bytes(), totalCols, iconRows))
	sb.WriteByte('\n')

	// Labels centred under each icon.
	for i, it := range items {
		if i > 0 {
			sb.WriteString(strings.Repeat(" ", gapCols))
		}
		lbl := it.name
		if len(lbl) > iconCols {
			lbl = lbl[:iconCols]
		}
		pad := iconCols - len(lbl)
		sb.WriteString(strings.Repeat(" ", pad/2))
		sb.WriteString(lbl)
		sb.WriteString(strings.Repeat(" ", pad-pad/2))
	}
	sb.WriteByte('\n')
	return sb.String()
}

// kittyEncode encodes pngData as a Kitty graphics protocol APC sequence
// occupying cols×rows terminal cells. Automatically chunks large payloads.
func kittyEncode(pngData []byte, cols, rows int) string {
	encoded := base64.StdEncoding.EncodeToString(pngData)
	const maxChunk = 4096
	var sb strings.Builder
	for i := 0; i < len(encoded); i += maxChunk {
		end := i + maxChunk
		if end > len(encoded) {
			end = len(encoded)
		}
		chunk := encoded[i:end]
		m := "0"
		if end < len(encoded) {
			m = "1"
		}
		var params string
		if i == 0 {
			params = fmt.Sprintf("a=T,f=100,c=%d,r=%d,m=%s", cols, rows, m)
		} else {
			params = fmt.Sprintf("m=%s", m)
		}
		fmt.Fprintf(&sb, "\x1b_G%s;%s\x1b\\", params, chunk)
	}
	return sb.String()
}

// ── Half-block fallback ───────────────────────────────────────────────────────

// art, cols characters wide and cols lines tall. bgR/bgG/bgB are the terminal
// background colour used for alpha compositing. Returns an empty string if no
// logo exists for agent.
func Render(agent string, cols int, bgR, bgG, bgB uint8) string {
	img, ok := decodedImage(agent)
	if !ok {
		return ""
	}
	return renderHalfBlocks(img, cols, bgR, bgG, bgB)
}

// RenderLine returns a single-line "strip" of the logo, cols characters wide,
// sampling a horizontal band at the given vertical fraction (0.0–1.0) of the
// image. Useful for compact inline badges in list views.
func RenderLine(agent string, cols int, vFrac float64, bgR, bgG, bgB uint8) string {
	img, ok := decodedImage(agent)
	if !ok {
		return strings.Repeat(" ", cols)
	}
	bounds := img.Bounds()
	srcW := bounds.Max.X - bounds.Min.X
	srcH := bounds.Max.Y - bounds.Min.Y
	rows := cols // consistent with renderHalfBlocks: square aspect ratio
	y := int(vFrac * float64(rows))
	if y >= rows-1 {
		y = rows - 2
	}
	var sb strings.Builder
	for x := 0; x < cols; x++ {
		sx0 := bounds.Min.X + x*srcW/cols
		sx1 := bounds.Min.X + (x+1)*srcW/cols
		sy0 := bounds.Min.Y + y*srcH/rows
		sy1 := bounds.Min.Y + (y+1)*srcH/rows
		sy2 := bounds.Min.Y + (y+2)*srcH/rows
		tr, tg, tb := avgRegion(img, sx0, sy0, sx1, sy1, bgR, bgG, bgB)
		br, bg_, bb := avgRegion(img, sx0, sy1, sx1, sy2, bgR, bgG, bgB)
		sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀", tr, tg, tb, br, bg_, bb))
	}
	sb.WriteString("\x1b[0m")
	return sb.String()
}

// Gallery renders logos for each found agent side by side, with name labels
// below. agents is the list of agent names to show. cols is the width of each
// icon. Returns a multi-line string ready to print.
func Gallery(agents []string, cols int, bgR, bgG, bgB uint8) string {
	type rendered struct {
		lines []string
		name  string
	}
	// Each icon is cols/2 terminal lines tall (rows=cols → cols/2 output lines).
	iconLines := cols / 2
	var items []rendered
	for _, a := range agents {
		r := Render(a, cols, bgR, bgG, bgB)
		if r == "" {
			placeholder := strings.Repeat("▒", cols)
			lines := make([]string, iconLines)
			for i := range lines {
				lines[i] = placeholder
			}
			items = append(items, rendered{lines: lines, name: a})
			continue
		}
		lines := strings.Split(strings.TrimRight(r, "\n"), "\n")
		items = append(items, rendered{lines: lines, name: a})
	}
	if len(items) == 0 {
		return ""
	}
	iconRows := len(items[0].lines)
	gap := "  "
	var sb strings.Builder
	for row := 0; row < iconRows; row++ {
		for i, it := range items {
			if i > 0 {
				sb.WriteString(gap)
			}
			if row < len(it.lines) {
				sb.WriteString(it.lines[row])
			}
		}
		sb.WriteByte('\n')
	}
	// Name labels
	for i, it := range items {
		if i > 0 {
			sb.WriteString(gap)
		}
		label := it.name
		if len(label) > cols {
			label = label[:cols]
		}
		pad := cols - len(label)
		sb.WriteString(strings.Repeat(" ", pad/2))
		sb.WriteString(label)
		sb.WriteString(strings.Repeat(" ", pad-pad/2))
	}
	sb.WriteByte('\n')
	return sb.String()
}

// GalleryAdaptive renders a half-block gallery, automatically adapting the
// icon width to fill the current terminal without wrapping.
func GalleryAdaptive(agents []string, bgR, bgG, bgB uint8) string {
	cols := adaptiveCols(len(agents))
	return Gallery(agents, cols, bgR, bgG, bgB)
}

// GalleryKittyAdaptive renders a Kitty-protocol gallery with icon size
// adapted to the current terminal width.
func GalleryKittyAdaptive(agents []string) string {
	iconCols := adaptiveCols(len(agents))
	return GalleryKitty(agents, iconCols, iconCols/2)
}

// adaptiveCols returns the optimal icon column width for a gallery of
// numAgents icons in the current terminal. The result is always even
// (required by the half-block renderer which steps rows by 2).
func adaptiveCols(numAgents int) int {
	if numAgents <= 0 {
		return 12
	}
	w := terminalWidth()
	const gapW = 2 // chars between icons
	cols := (w - gapW*(numAgents-1)) / numAgents
	if cols < 8 {
		cols = 8
	}
	if cols > 22 {
		cols = 22
	}
	return cols &^ 1 // round down to even so rows=cols covers the full image
}

// terminalWidth returns the current terminal column count, queried via
// charmbracelet/x/term. Falls back to the COLUMNS env var, then 80.
func terminalWidth() int {
	if w, _, err := charmbterm.GetSize(os.Stdout.Fd()); err == nil && w > 0 {
		return w
	}
	if s := os.Getenv("COLUMNS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return 80
}

func renderHalfBlocks(img image.Image, cols int, bgR, bgG, bgB uint8) string {
	bounds := img.Bounds()
	srcW := bounds.Max.X - bounds.Min.X
	srcH := bounds.Max.Y - bounds.Min.Y
	// rows = cols (not cols*2) so the output is visually square: each terminal
	// character cell is ~2:1 (height:width), and half-blocks give 2 vertical
	// "pixels" per row, so rows=cols yields cols chars wide × cols/2 lines tall,
	// which is cols×cw wide and cols/2 × 2cw = cols×cw tall → square.
	rows := cols
	var sb strings.Builder
	for y := 0; y < rows-1; y += 2 {
		for x := 0; x < cols; x++ {
			sx0 := bounds.Min.X + x*srcW/cols
			sx1 := bounds.Min.X + (x+1)*srcW/cols
			sy0 := bounds.Min.Y + y*srcH/rows
			sy1 := bounds.Min.Y + (y+1)*srcH/rows
			sy2 := bounds.Min.Y + (y+2)*srcH/rows
			tr, tg, tb := avgRegion(img, sx0, sy0, sx1, sy1, bgR, bgG, bgB)
			br, bg_, bb := avgRegion(img, sx0, sy1, sx1, sy2, bgR, bgG, bgB)
			sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀", tr, tg, tb, br, bg_, bb))
		}
		sb.WriteString("\x1b[0m\n")
	}
	return sb.String()
}

// avgRegion averages composited pixel colours in source region [sx0,sx1)×[sy0,sy1).
// At least one pixel is always sampled (clamps empty regions to a single pixel).
func avgRegion(img image.Image, sx0, sy0, sx1, sy1 int, bgR, bgG, bgB uint8) (uint8, uint8, uint8) {
	if sx1 <= sx0 {
		sx1 = sx0 + 1
	}
	if sy1 <= sy0 {
		sy1 = sy0 + 1
	}
	var rSum, gSum, bSum float64
	count := 0
	for py := sy0; py < sy1; py++ {
		for px := sx0; px < sx1; px++ {
			r, g, b := composite(img.At(px, py), bgR, bgG, bgB)
			rSum += float64(r)
			gSum += float64(g)
			bSum += float64(b)
			count++
		}
	}
	return uint8(rSum / float64(count)), uint8(gSum / float64(count)), uint8(bSum / float64(count))
}

func composite(c color.Color, bgR, bgG, bgB uint8) (uint8, uint8, uint8) {
	r, g, b, a := c.RGBA()
	// c.RGBA() returns alpha-premultiplied values in [0, 65535].
	// r>>8 is already actual_r*alpha in [0,255]. The correct Porter-Duff "over"
	// composite with premultiplied source is: out = src_premul + bg*(1−alpha).
	af := float64(a) / 65535.0
	return uint8(float64(r>>8) + float64(bgR)*(1-af)),
		uint8(float64(g>>8) + float64(bgG)*(1-af)),
		uint8(float64(b>>8) + float64(bgB)*(1-af))
}
