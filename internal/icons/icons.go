// Package icons provides agent logo rendering for terminal display.
// Logos are embedded as 64×64 RGBA PNGs and rendered as Unicode half-block
// (▀/▄) art using true-colour ANSI escape sequences. Each character cell
// represents two vertical pixels, so Render(agent, cols, ...) produces a
// block cols characters wide and cols characters tall.
package icons

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"strings"
	"sync"
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

// Render returns a multi-line ANSI string showing agent's logo as half-block
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
	rows := cols * 2
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
	var items []rendered
	for _, a := range agents {
		r := Render(a, cols, bgR, bgG, bgB)
		if r == "" {
			// Placeholder block for agents without an icon
			placeholder := strings.Repeat("▒", cols)
			lines := make([]string, cols)
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

func renderHalfBlocks(img image.Image, cols int, bgR, bgG, bgB uint8) string {
	bounds := img.Bounds()
	srcW := bounds.Max.X - bounds.Min.X
	srcH := bounds.Max.Y - bounds.Min.Y
	rows := cols * 2
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
