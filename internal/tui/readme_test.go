package tui

import (
	"strings"
	"testing"
)

func TestCleanReadme(t *testing.T) {
	in := "<p align=\"center\">\n  <img src=\"x.png\" width=\"1\" />\n</p>\n\n" +
		"<h1 align=\"center\">caveman</h1>\n\n" +
		"<p align=\"center\">\n  <a href=\"#install\">Install</a> &bull;\n  <a href=\"./X.md\">Guide</a>\n</p>\n\n" +
		"[![Stars](a.svg)](b)\n\nReal text <https://x.y> and Vec<T>.\n\n```html\n<p align=\"center\">keep</p>\n```\n"
	got := string(cleanReadme([]byte(in)))
	for _, want := range []string{"# caveman", "Install •\nGuide", "Real text <https://x.y> and Vec<T>.", "<p align=\"center\">keep</p>"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	for _, bad := range []string{"<img", "[![Stars]", "<a href"} {
		if strings.Contains(got, bad) {
			t.Errorf("unexpected %q in:\n%s", bad, got)
		}
	}
	if strings.HasPrefix(got, "\n") {
		t.Errorf("leading blank line")
	}
}
