package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alecthomas/chroma/v2"
	chromaformatters "github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func readPreview(path string, isDir bool, maxLines, maxWidth int) []string {
	if isDir {
		entries, err := os.ReadDir(path)
		if err != nil {
			return []string{dimStyle.Render("  " + err.Error())}
		}
		lines := make([]string, 0, min(len(entries), maxLines))
		for _, e := range entries {
			if len(lines) >= maxLines {
				break
			}
			name := e.Name()
			if e.IsDir() {
				name += "/"
			}
			lines = append(lines, previewStyle.Render(" "+truncateRunes(name, maxWidth-1)))
		}
		return lines
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return []string{dimStyle.Render("  " + err.Error())}
	}

	// Binary detection via null-byte scan of the first 512 bytes.
	check := data
	if len(check) > 512 {
		check = check[:512]
	}
	if bytes.IndexByte(check, 0) >= 0 {
		return []string{dimStyle.Render("  [binary file]")}
	}

	if strings.EqualFold(filepath.Base(path), "README.md") {
		data = cleanReadme(data)
	}

	if lines := syntaxHighlight(path, data, maxLines, maxWidth); lines != nil {
		return lines
	}

	rawLines := strings.Split(string(data), "\n")
	result := make([]string, 0, min(len(rawLines), maxLines))
	for i, line := range rawLines {
		if i >= maxLines {
			break
		}
		result = append(result, previewStyle.Render(" "+truncateRunes(line, maxWidth-1)))
	}
	return result
}

// splitFrontmatter splits data into a YAML frontmatter block and the remaining
// body when the file begins with a --- delimiter. Returns ok=false if no
// frontmatter is present.
func splitFrontmatter(data []byte) (fm, body []byte, ok bool) {
	lines := strings.Split(string(data), "\n")
	if len(lines) < 2 || strings.TrimRight(lines[0], "\r") != "---" {
		return nil, nil, false
	}
	for i := 1; i < len(lines); i++ {
		trimmed := strings.TrimRight(lines[i], "\r")
		if trimmed == "---" || trimmed == "..." {
			fm = []byte(strings.Join(lines[:i+1], "\n") + "\n")
			body = []byte(strings.Join(lines[i+1:], "\n"))
			return fm, body, true
		}
	}
	return nil, nil, false
}

// highlightBlock runs Chroma on data using the given lexer and returns rendered
// lines, each prefixed with a space. Returns nil on any failure.
func highlightBlock(lexer chroma.Lexer, data []byte, maxLines, maxWidth int) []string {
	lexer = chroma.Coalesce(lexer)
	style := styles.Get("monokai")
	if style == nil {
		return nil
	}
	tokens, err := lexer.Tokenise(nil, string(data))
	if err != nil {
		return nil
	}
	var buf bytes.Buffer
	if err := chromaformatters.TTY16m.Format(&buf, style, tokens); err != nil {
		return nil
	}
	const reset = "\033[0m"
	rawLines := strings.Split(buf.String(), "\n")
	result := make([]string, 0, min(len(rawLines), maxLines))
	for i, line := range rawLines {
		if i >= maxLines {
			break
		}
		if lipgloss.Width(line) > maxWidth-1 {
			line = ansi.Truncate(line, maxWidth-1, "")
		}
		result = append(result, " "+line+reset)
	}
	return result
}

func syntaxHighlight(path string, data []byte, maxLines, maxWidth int) []string {
	// For markdown files, highlight YAML frontmatter separately so the --- block
	// is not misread as a horizontal rule.
	if ext := strings.ToLower(filepath.Ext(path)); ext == ".md" || ext == ".markdown" {
		if fm, body, hasFM := splitFrontmatter(data); hasFM {
			if yamlLexer := lexers.Get("yaml"); yamlLexer != nil {
				fmLines := highlightBlock(yamlLexer, fm, maxLines, maxWidth)
				if fmLines != nil {
					remaining := maxLines - len(fmLines)
					if remaining > 0 && len(strings.TrimSpace(string(body))) > 0 {
						if mdLexer := lexers.Get("markdown"); mdLexer != nil {
							if mdLines := highlightBlock(mdLexer, body, remaining, maxWidth); mdLines != nil {
								return append(fmLines, mdLines...)
							}
						}
					}
					return fmLines
				}
			}
		}
	}

	lexer := lexers.Match(path)
	if lexer == nil {
		lexer = lexers.Analyse(string(data))
	}
	if lexer == nil {
		return nil
	}
	return highlightBlock(lexer, data, maxLines, maxWidth)
}

var treeDirStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("110"))

// renderTree draws root's contents as a connector tree, dirs first, up to maxDepth levels.
func renderTree(root string, maxDepth int) []string {
	var lines []string
	var walk func(dir, prefix string, depth int)
	walk = func(dir, prefix string, depth int) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		visible := entries[:0:0]
		for _, e := range entries {
			if n := e.Name(); strings.HasPrefix(n, ".") || n == "node_modules" {
				continue
			}
			visible = append(visible, e)
		}
		sort.SliceStable(visible, func(i, j int) bool {
			return visible[i].IsDir() && !visible[j].IsDir()
		})
		for i, e := range visible {
			last := i == len(visible)-1
			branch, next := "├── ", "│   "
			if last {
				branch, next = "└── ", "    "
			}
			name := previewStyle.Render(e.Name())
			if e.IsDir() {
				name = treeDirStyle.Render(e.Name() + "/")
			}
			lines = append(lines, " "+statusAbsentStyle.Render(prefix+branch)+name)
			if e.IsDir() && depth < maxDepth {
				walk(filepath.Join(dir, e.Name()), prefix+next, depth+1)
			}
		}
	}
	walk(root, "", 1)
	return lines
}

// pluginDoc picks the most informative markdown file in a plugin dir:
// README, then the first skill, agent, command, or root-level .md.
func pluginDoc(root string) string {
	readme := filepath.Join(root, "README.md")
	if _, err := os.Stat(readme); err == nil {
		return readme
	}
	if dirs, err := os.ReadDir(filepath.Join(root, "skills")); err == nil {
		for _, d := range dirs {
			f := filepath.Join(root, "skills", d.Name(), "SKILL.md")
			if _, err := os.Stat(f); err == nil {
				return f
			}
		}
	}
	for _, sub := range []string{"agents", "commands", ""} {
		entries, err := os.ReadDir(filepath.Join(root, sub))
		if err != nil {
			continue
		}
		for _, e := range entries {
			n := strings.ToLower(e.Name())
			if e.IsDir() || !strings.HasSuffix(n, ".md") || strings.HasPrefix(n, "changelog") || strings.HasPrefix(n, "license") {
				continue
			}
			return filepath.Join(root, sub, e.Name())
		}
	}
	return ""
}
