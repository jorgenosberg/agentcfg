package tui

import (
	"html"
	"regexp"
	"strings"
)

const htmlMark = "\x00"

const htmlTags = "p|div|a|span|picture|source|img|br|hr|h[1-6]|strong|b|em|i|sub|sup|small|center|details|summary|table|thead|tbody|tr|td|th|ul|ol|li|code|kbd"

var (
	reHTMLComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	reHTMLHeading = regexp.MustCompile(`(?is)<h([1-6])(?:\s[^>]*)?>(.*?)</h[1-6]>`)
	reHTMLDrop    = regexp.MustCompile(`(?i)</?(?:img|source|picture|br|hr)(?:\s[^>]*)?/?>`)
	reHTMLTag     = regexp.MustCompile(`(?i)</?(?:` + htmlTags + `)(?:\s[^>]*)?/?>`)
	reBadgeOnly   = regexp.MustCompile(`^\s*(?:(?:\[!\[[^\]]*\]\([^)]*\)\]\([^)]*\)|!\[[^\]]*\]\([^)]*\))\s*)+$`)
	reBlankRuns   = regexp.MustCompile(`\n{3,}`)
)

// cleanReadme strips decorative HTML (centered headers, logos, badges) from a
// README so the text-only preview starts with real content. Fenced code is left alone.
func cleanReadme(data []byte) []byte {
	var out, seg []string
	flush := func() {
		if len(seg) > 0 {
			out = append(out, cleanReadmeText(strings.Join(seg, "\n")))
			seg = seg[:0]
		}
	}
	inFence := false
	var fence []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimLeft(line, " "), "```") {
			if !inFence {
				flush()
			}
			inFence = !inFence
			fence = append(fence, line)
			if !inFence {
				out = append(out, strings.Join(fence, "\n"))
				fence = nil
			}
			continue
		}
		if inFence {
			fence = append(fence, line)
		} else {
			seg = append(seg, line)
		}
	}
	flush()
	if len(fence) > 0 {
		out = append(out, strings.Join(fence, "\n"))
	}
	res := reBlankRuns.ReplaceAllString(strings.Join(out, "\n"), "\n\n")
	return []byte(strings.TrimLeft(res, "\n"))
}

func cleanReadmeText(s string) string {
	s = reHTMLComment.ReplaceAllString(s, htmlMark)
	s = reHTMLHeading.ReplaceAllStringFunc(s, func(m string) string {
		sub := reHTMLHeading.FindStringSubmatch(m)
		return htmlMark + strings.Repeat("#", int(sub[1][0]-'0')) + " " + strings.TrimSpace(sub[2])
	})
	s = reHTMLDrop.ReplaceAllString(s, htmlMark)
	s = reHTMLTag.ReplaceAllString(s, htmlMark)
	if !strings.Contains(s, htmlMark) {
		return s
	}
	s = html.UnescapeString(s)
	lines := strings.Split(s, "\n")
	res := lines[:0]
	for _, l := range lines {
		if strings.Contains(l, htmlMark) {
			l = strings.TrimSpace(strings.ReplaceAll(l, htmlMark, ""))
			if l == "" {
				res = append(res, "")
				continue
			}
		}
		if reBadgeOnly.MatchString(l) {
			continue
		}
		res = append(res, l)
	}
	return strings.Join(res, "\n")
}
