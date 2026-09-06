// Command sitegen builds the ironwail-go GitHub Pages site from the article
// markdown and project metadata. It produces a static site in site/ with an
// index page (project showcase) and an article page (the full development
// article rendered to HTML).
package main

import (
	"bufio"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	repoRoot := findRepoRoot()
	articlePath := filepath.Join(repoRoot, "article", "ironwail_go.md")
	readmePath := filepath.Join(repoRoot, "README.md")
	outDir := filepath.Join(repoRoot, "site")

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", outDir, err)
		os.Exit(1)
	}

	articleMD, err := os.ReadFile(articlePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read article: %v\n", err)
		os.Exit(1)
	}
	readmeMD, err := os.ReadFile(readmePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read readme: %v\n", err)
		os.Exit(1)
	}

	articleHTML := markdownToHTML(string(articleMD), "article")
	readmeHTML := markdownToHTML(stripTitleHeading(string(readmeMD)), "")

	data := templateData{
		ArticleHTML: template.HTML(articleHTML),
		ReadmeHTML:  template.HTML(readmeHTML),
		ArticleTOC:  buildArticleTOC(string(articleMD)),
		Packages:    countPackages(repoRoot),
		Chapters:    countChapters(string(articleMD)),
		GoLOC:       formatGoLOC(countGoLines(repoRoot)),
	}

	for _, page := range pages {
		outPath := filepath.Join(outDir, page.filename)
		f, err := os.Create(outPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create %s: %v\n", outPath, err)
			os.Exit(1)
		}
		tmpl, err := template.New(page.filename).Parse(page.tmpl)
		if err != nil {
			_ = f.Close()
			fmt.Fprintf(os.Stderr, "parse template %s: %v\n", page.filename, err)
			os.Exit(1)
		}
		if err := tmpl.Execute(f, data); err != nil {
			_ = f.Close()
			fmt.Fprintf(os.Stderr, "execute template %s: %v\n", page.filename, err)
			os.Exit(1)
		}
		_ = f.Close()
		fmt.Printf("wrote %s\n", outPath)
	}
}

type templateData struct {
	ArticleHTML template.HTML
	ReadmeHTML  template.HTML
	ArticleTOC  []tocEntry
	Packages    int
	Chapters    int
	GoLOC       string
}

// tocEntry is one link in the article sidebar, derived from the article
// markdown headings.
type tocEntry struct {
	Class string
	Text  string
	ID    string
}

type pageSpec struct {
	filename string
	tmpl     string
}

var pages = []pageSpec{
	{"index.html", indexTemplate},
	{"article.html", articleTemplate},
}

func findRepoRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			fmt.Fprintln(os.Stderr, "could not find repo root (go.mod)")
			os.Exit(1)
		}
		dir = parent
	}
}

// stripTitleHeading removes the document's own first H1 heading, so the
// site hero can own the page title.
func stripTitleHeading(md string) string {
	parts := strings.SplitN(md, "\n", 2)
	if len(parts) == 2 && strings.HasPrefix(parts[0], "# ") {
		return strings.TrimPrefix(parts[1], "\n")
	}
	return md
}

// countChapters counts the # Chapter headings in the article.
func countChapters(articleMD string) int {
	re := regexp.MustCompile(`(?m)^# Chapter `)
	return len(re.FindAllStringIndex(articleMD, -1))
}

// buildArticleTOC derives the article sidebar from the markdown headings.
// It keeps the top two levels and skips code blocks and the article's own
// table of contents heading.
func buildArticleTOC(articleMD string) []tocEntry {
	var entries []tocEntry
	scanner := bufio.NewScanner(strings.NewReader(articleMD))
	inCode := false
	skippedTitle := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "```") {
			inCode = !inCode
			continue
		}
		if inCode {
			continue
		}
		text, level := parseHeading(line)
		if text == "" || level > 2 || text == "Table of Contents" {
			continue
		}
		if level == 1 && !skippedTitle {
			// The first H1 is the document title, not a section.
			skippedTitle = true
			continue
		}
		cls := "toc-h1"
		if level == 2 {
			cls = "toc-h2"
		}
		entries = append(entries, tocEntry{Class: cls, Text: text, ID: headingToID(text)})
	}
	return entries
}

// countPackages counts the Go packages of the root module via go list.
// It falls back to zero when the Go toolchain is not available.
func countPackages(repoRoot string) int {
	cmd := exec.Command("go", "list", "./...")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "go list: %v\n", err)
		return 0
	}
	n := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

// countGoLines sums the lines of every .go file in the tree, skipping
// scratch and build output directories.
func countGoLines(repoRoot string) int {
	total := 0
	_ = filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == ".tmp" || d.Name() == ".gograph" || d.Name() == "site" || d.Name() == "bin" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			total++
		}
		_ = f.Close()
		return nil
	})
	return total
}

// formatGoLOC renders a line count as a compact thousands shorthand.
func formatGoLOC(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%dk", n/1000)
	}
	return fmt.Sprintf("%d", n)
}

// linkBaseDir is the repo-relative directory of the markdown being
// converted, used to remap relative links to GitHub blob URLs.
var linkBaseDir string

// markdownToHTML converts a markdown document to HTML for the site. It
// handles the specific patterns used in the article: headings, fenced code
// blocks, tables, links, bold/italic, lists, horizontal rules, anchor tags,
// and blockquotes. The baseDir is the repo-relative directory the markdown
// lives in; relative links from that directory are remapped to absolute
// GitHub blob URLs, because the generated site does not sit next to the
// repository files. Absolute URLs, mailto links, and in-page anchors pass
// through unchanged.
func markdownToHTML(md, baseDir string) string {
	linkBaseDir = baseDir

	// First pass: collect reference-style link definitions [label]: url
	refLinks := map[string]string{}
	refLinkRe := regexp.MustCompile(`^\[([^\]]+)\]:\s*<?([^\s>]+)>?(?:\s+.*)?$`)
	scanner := bufio.NewScanner(strings.NewReader(md))
	for scanner.Scan() {
		if m := refLinkRe.FindStringSubmatch(strings.TrimSpace(scanner.Text())); m != nil {
			refLinks[strings.ToLower(strings.TrimSpace(m[1]))] = strings.TrimSpace(m[2])
		}
	}

	var b strings.Builder
	scanner = bufio.NewScanner(strings.NewReader(md))

	inCodeBlock := false
	codeLang := ""
	var codeLines []string
	inTable := false
	var tableRows []string

	flushTable := func() {
		if !inTable || len(tableRows) == 0 {
			return
		}
		b.WriteString(renderTable(tableRows, refLinks))
		tableRows = nil
		inTable = false
	}

	for scanner.Scan() {
		line := scanner.Text()

		// Skip reference-style link definitions (already collected)
		if refLinkRe.MatchString(strings.TrimSpace(line)) {
			continue
		}

		// Fenced code blocks
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				b.WriteString(`<pre><code class="language-` + template.HTMLEscapeString(codeLang) + `">`)
				b.WriteString(template.HTMLEscapeString(strings.Join(codeLines, "\n")))
				b.WriteString("</code></pre>\n")
				codeLines = nil
				inCodeBlock = false
				codeLang = ""
			} else {
				flushTable()
				inCodeBlock = true
				codeLang = strings.TrimSpace(strings.TrimPrefix(line, "```"))
			}
			continue
		}
		if inCodeBlock {
			codeLines = append(codeLines, line)
			continue
		}

		// Tables
		if strings.HasPrefix(line, "|") && strings.HasSuffix(strings.TrimSpace(line), "|") {
			if !inTable {
				inTable = true
			}
			tableRows = append(tableRows, line)
			continue
		}
		if inTable {
			flushTable()
		}

		// Horizontal rules
		if strings.TrimSpace(line) == "---" || strings.TrimSpace(line) == "***" {
			b.WriteString("<hr>\n")
			continue
		}

		// Headings
		if heading, level := parseHeading(line); heading != "" {
			id := headingToID(heading)
			fmt.Fprintf(&b, `<h%d id="%s">%s</h%d>`+"\n", level, id, inlineMarkdown(heading, refLinks), level)
			continue
		}

		// Anchor tags (<a id="..."></a>)
		if strings.HasPrefix(line, "<a ") {
			b.WriteString(line + "\n")
			continue
		}

		// Blockquotes
		if strings.HasPrefix(line, "> ") {
			content := strings.TrimPrefix(line, "> ")
			b.WriteString("<blockquote>" + inlineMarkdown(content, refLinks) + "</blockquote>\n")
			continue
		}

		// Unordered list items
		if ulMatch := regexp.MustCompile(`^(\s*)[-*]\s+(.*)$`).FindStringSubmatch(line); ulMatch != nil {
			b.WriteString("<li>" + inlineMarkdown(ulMatch[2], refLinks) + "</li>\n")
			continue
		}

		// Ordered list items
		if olMatch := regexp.MustCompile(`^(\s*)\d+\.\s+(.*)$`).FindStringSubmatch(line); olMatch != nil {
			b.WriteString("<li>" + inlineMarkdown(olMatch[2], refLinks) + "</li>\n")
			continue
		}

		// Empty lines
		if strings.TrimSpace(line) == "" {
			b.WriteString("\n")
			continue
		}

		// Paragraphs
		b.WriteString("<p>" + inlineMarkdown(line, refLinks) + "</p>\n")
	}

	flushTable()
	return b.String()
}

func parseHeading(line string) (string, int) {
	for i := 6; i >= 1; i-- {
		prefix := strings.Repeat("#", i) + " "
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix), i
		}
	}
	return "", 0
}

func headingToID(heading string) string {
	s := strings.ToLower(heading)
	s = regexp.MustCompile(`[^a-z0-9\s-]`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, "-")
	return s
}

func inlineMarkdown(s string, refLinks map[string]string) string {
	// Preserve raw HTML anchor tags through escaping
	var htmlTags []string
	s = regexp.MustCompile(`<a\s[^>]*>[^<]*</a>|<a\s[^>]*/?>`).ReplaceAllStringFunc(s, func(match string) string {
		idx := len(htmlTags)
		htmlTags = append(htmlTags, match)
		return fmt.Sprintf("%%HTMLTAG%d%%", idx)
	})

	s = template.HTMLEscapeString(s)

	// Restore preserved HTML tags
	for i, tag := range htmlTags {
		s = strings.Replace(s, fmt.Sprintf("%%HTMLTAG%d%%", i), tag, 1)
	}

	// Inline code (must come before bold/italic to avoid conflicts)
	s = regexp.MustCompile("`([^`]+)`").ReplaceAllString(s, `<code>$1</code>`)

	// Bold+italic
	s = regexp.MustCompile(`\*\*\*(.+?)\*\*\*`).ReplaceAllString(s, `<strong><em>$1</em></strong>`)
	// Bold
	s = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllString(s, `<strong>$1</strong>`)
	// Italic
	s = regexp.MustCompile(`\*(.+?)\*`).ReplaceAllString(s, `<em>$1</em>`)

	// Autolinks <https://...>
	s = regexp.MustCompile(`&lt;(https?://[^&>]+)&gt;`).ReplaceAllString(s, `<a href="$1">$1</a>`)

	// Links [text](url)
	s = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`).ReplaceAllStringFunc(s, func(match string) string {
		sub := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`).FindStringSubmatch(match)
		return `<a href="` + remapRepoLink(sub[2]) + `">` + sub[1] + `</a>`
	})

	// Reference-style links [text][label]
	if refLinks != nil {
		s = regexp.MustCompile(`\[([^\]]+)\]\[([^\]]*)\]`).ReplaceAllStringFunc(s, func(match string) string {
			parts := regexp.MustCompile(`\[([^\]]+)\]\[([^\]]*)\]`).FindStringSubmatch(match)
			text := parts[1]
			label := strings.ToLower(strings.TrimSpace(parts[2]))
			if label == "" {
				label = strings.ToLower(strings.TrimSpace(text))
			}
			if url, ok := refLinks[label]; ok {
				return `<a href="` + remapRepoLink(url) + `">` + text + `</a>`
			}
			return match
		})

		// Shortcut reference links [label]
		s = regexp.MustCompile(`\[([^\]]+)\]`).ReplaceAllStringFunc(s, func(match string) string {
			sub := regexp.MustCompile(`\[([^\]]+)\]`).FindStringSubmatch(match)
			label := strings.ToLower(strings.TrimSpace(sub[1]))
			if url, ok := refLinks[label]; ok {
				return `<a href="` + remapRepoLink(url) + `">` + sub[1] + `</a>`
			}
			return match
		})
	}

	return s
}

// remapRepoLink turns a repo-relative markdown link into an absolute
// GitHub blob URL. External URLs, mailto links, in-page anchors, and
// protocol-relative URLs pass through unchanged.
func remapRepoLink(url string) string {
	url = strings.TrimSpace(url)
	url = strings.TrimPrefix(url, "<")
	url = strings.TrimSuffix(url, ">")

	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") ||
		strings.HasPrefix(url, "//") || strings.HasPrefix(url, "mailto:") ||
		strings.HasPrefix(url, "#") {
		return url
	}
	clean := strings.TrimPrefix(url, "./")
	if clean == "index.html" || clean == "article.html" {
		return clean
	}
	rel := path.Clean(path.Join(linkBaseDir, url))
	if rel == "." || rel == "" {
		return url
	}
	return "https://github.com/darkliquid/ironwail-go/blob/main/" + rel
}

func renderTable(rows []string, refLinks map[string]string) string {
	if len(rows) < 2 {
		return ""
	}

	parseRow := func(row string) []string {
		row = strings.TrimSpace(row)
		row = strings.Trim(row, "|")
		cells := strings.Split(row, "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		return cells
	}

	isSeparator := func(row string) bool {
		return regexp.MustCompile(`^[\s|:-]+$`).MatchString(row)
	}

	var b strings.Builder
	b.WriteString("<table>\n")

	headerCells := parseRow(rows[0])
	b.WriteString("<thead><tr>")
	for _, cell := range headerCells {
		b.WriteString("<th>" + inlineMarkdown(cell, refLinks) + "</th>")
	}
	b.WriteString("</tr></thead>\n<tbody>\n")

	for _, row := range rows[1:] {
		if isSeparator(row) {
			continue
		}
		cells := parseRow(row)
		b.WriteString("<tr>")
		for _, cell := range cells {
			b.WriteString("<td>" + inlineMarkdown(cell, refLinks) + "</td>")
		}
		b.WriteString("</tr>\n")
	}

	b.WriteString("</tbody></table>\n")
	return b.String()
}
