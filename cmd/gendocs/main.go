// Command gendocs generates the CLI reference for the docs site from the
// live Cobra command tree, so the reference can never drift from the
// actual flags and commands. Run via `make gen-docs`; `make check-docs`
// fails CI if the committed output is stale.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra/doc"

	"github.com/jorgenosberg/agentcfg/internal/cli"
)

const outDir = "docs/src/content/docs/reference/cli"

func main() {
	root := cli.NewRoot()
	root.DisableAutoGenTag = true

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "gendocs:", err)
		os.Exit(1)
	}

	// filePrepender adds Starlight frontmatter above cobra's generated body.
	filePrepender := func(filename string) string {
		title := strings.ReplaceAll(strings.TrimSuffix(filepath.Base(filename), ".md"), "_", " ")
		return fmt.Sprintf(
			"---\ntitle: %s\ndescription: agentcfg CLI reference for `%s`\neditUrl: false\n---\n\n",
			title, title,
		)
	}

	// linkHandler rewrites cobra's relative "name.md" cross-links (used in
	// "SEE ALSO") into links Starlight can actually follow. All generated
	// files sit flat in outDir, but Starlight serves each as a trailing-slash
	// "directory" URL (.../agentcfg_target_add/), so a same-level sibling
	// link must go up one level first: "agentcfg_target.md" -> "../agentcfg_target/".
	linkHandler := func(link string) string {
		name := strings.TrimSuffix(link, ".md")
		return "../" + name + "/"
	}

	if err := doc.GenMarkdownTreeCustom(root, outDir, filePrepender, linkHandler); err != nil {
		fmt.Fprintln(os.Stderr, "gendocs:", err)
		os.Exit(1)
	}

	// Cobra opens every file with an "## <command>" heading that duplicates
	// the frontmatter title Starlight already renders as the page H1.
	if err := normalizeHeadings(outDir); err != nil {
		fmt.Fprintln(os.Stderr, "gendocs:", err)
		os.Exit(1)
	}
	fmt.Println("wrote CLI reference to", outDir)
}

// normalizeHeadings removes the first "## " heading (and a blank line
// following it) from every generated Markdown file, then promotes the
// remaining "### " sections to "## " so each page keeps an h1 → h2
// hierarchy under the frontmatter title.
func normalizeHeadings(dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return err
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		lines := strings.Split(string(b), "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "## ") {
				rest := lines[i+1:]
				if len(rest) > 0 && rest[0] == "" {
					rest = rest[1:]
				}
				lines = append(lines[:i], rest...)
				break
			}
		}
		inFence := false
		for i, line := range lines {
			if strings.HasPrefix(line, "```") {
				inFence = !inFence
			}
			if !inFence && strings.HasPrefix(line, "### ") {
				lines[i] = line[1:]
			}
		}
		if err := os.WriteFile(f, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
			return err
		}
	}
	return nil
}
