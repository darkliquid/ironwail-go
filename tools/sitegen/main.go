// Command sitegen builds the ironwail-go GitHub Pages site from the article
// markdown and project metadata. It produces a static site in site/ with an
// index page (project showcase) and an article page (the full development
// article rendered to HTML).
package main

import (
	"bufio"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	repoRoot := findRepoRoot()
	articlePath := filepath.Join(repoRoot, "article", "ironwail_go.md")
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

	articleHTML := markdownToHTML(string(articleMD))

	data := templateData{
		ArticleHTML: template.HTML(articleHTML),
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
			f.Close()
			fmt.Fprintf(os.Stderr, "parse template %s: %v\n", page.filename, err)
			os.Exit(1)
		}
		if err := tmpl.Execute(f, data); err != nil {
			f.Close()
			fmt.Fprintf(os.Stderr, "execute template %s: %v\n", page.filename, err)
			os.Exit(1)
		}
		f.Close()
		fmt.Printf("wrote %s\n", outPath)
	}
}

type templateData struct {
	ArticleHTML template.HTML
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

// markdownToHTML converts the article's markdown to HTML. It handles the
// specific patterns used in the article: headings, fenced code blocks,
// tables, links, bold/italic, lists, horizontal rules, anchor tags, and
// blockquotes.
func markdownToHTML(md string) string {
	// First pass: collect reference-style link definitions [label]: url
	refLinks := map[string]string{}
	refLinkRe := regexp.MustCompile(`^\[([^\]]+)\]:\s+(.+)$`)
	scanner := bufio.NewScanner(strings.NewReader(md))
	for scanner.Scan() {
		if m := refLinkRe.FindStringSubmatch(scanner.Text()); m != nil {
			refLinks[strings.ToLower(m[1])] = strings.TrimSpace(m[2])
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
		if refLinkRe.MatchString(line) {
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
			b.WriteString(fmt.Sprintf(`<h%d id="%s">%s</h%d>`+"\n", level, id, inlineMarkdown(heading, refLinks), level))
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

	// Links [text](url)
	s = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`).ReplaceAllString(s, `<a href="$2">$1</a>`)

	// Reference-style links [text][label]
	if refLinks != nil {
		s = regexp.MustCompile(`\[([^\]]+)\]\[([^\]]*)\]`).ReplaceAllStringFunc(s, func(match string) string {
			parts := regexp.MustCompile(`\[([^\]]+)\]\[([^\]]*)\]`).FindStringSubmatch(match)
			text := parts[1]
			label := strings.ToLower(parts[2])
			if label == "" {
				label = strings.ToLower(text)
			}
			if url, ok := refLinks[label]; ok {
				return `<a href="` + url + `">` + text + `</a>`
			}
			return match
		})
	}

	return s
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
