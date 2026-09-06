package main

import (
	"strings"
	"testing"
)

func TestMarkdownToHTML_LinkConversion(t *testing.T) {
	input := `# Title

The primary source is [Ironwail][1], and [Quake][2] is the original.
Also see [Ericw Tools][ericw] and shortcut [Oto].
Here is an inline link to [Docs](docs/LEARNING_GUIDE.md) and [Article](article.html).
Check out <https://github.com/darkliquid/ironwail-go>.

[1]:https://github.com/andrei-drexler/ironwail
[2]: https://github.com/id-Software/Quake
[ericw]: <https://github.com/ericwa/ericw-tools> "Ericw Tools"
[oto]:https://github.com/ebitengine/oto
`

	html := markdownToHTML(input, "")

	// Reference links without space after colon: [1]:https://...
	if !strings.Contains(html, `<a href="https://github.com/andrei-drexler/ironwail">Ironwail</a>`) {
		t.Errorf("expected Ironwail link to be converted, got:\n%s", html)
	}

	// Reference links with space after colon: [2]: https://...
	if !strings.Contains(html, `<a href="https://github.com/id-Software/Quake">Quake</a>`) {
		t.Errorf("expected Quake link to be converted, got:\n%s", html)
	}

	// Reference links with angle brackets and title: [ericw]: <...> "..."
	if !strings.Contains(html, `<a href="https://github.com/ericwa/ericw-tools">Ericw Tools</a>`) {
		t.Errorf("expected Ericw Tools link to be converted, got:\n%s", html)
	}

	// Shortcut reference links: [Oto] -> [oto]:https://...
	if !strings.Contains(html, `<a href="https://github.com/ebitengine/oto">Oto</a>`) {
		t.Errorf("expected shortcut Oto link to be converted, got:\n%s", html)
	}

	// Repo relative inline link: docs/LEARNING_GUIDE.md
	if !strings.Contains(html, `<a href="https://github.com/darkliquid/ironwail-go/blob/main/docs/LEARNING_GUIDE.md">Docs</a>`) {
		t.Errorf("expected repo relative Docs link to be remapped to GitHub blob URL, got:\n%s", html)
	}

	// Site relative link: article.html preserved
	if !strings.Contains(html, `<a href="article.html">Article</a>`) {
		t.Errorf("expected site internal link article.html to be preserved, got:\n%s", html)
	}

	// Autolinks: <https://...>
	if !strings.Contains(html, `<a href="https://github.com/darkliquid/ironwail-go">https://github.com/darkliquid/ironwail-go</a>`) {
		t.Errorf("expected autolink to be converted, got:\n%s", html)
	}

	// Definitions should not appear as rendered paragraphs
	if strings.Contains(html, `<p>[1]:`) || strings.Contains(html, `[ericw]:`) {
		t.Errorf("expected reference link definitions to be stripped from output, got:\n%s", html)
	}
}

func TestRemapRepoLink(t *testing.T) {
	linkBaseDir = "article"
	defer func() { linkBaseDir = "" }()

	tests := []struct {
		input string
		want  string
	}{
		{"https://example.com", "https://example.com"},
		{"http://example.com", "http://example.com"},
		{"mailto:test@example.com", "mailto:test@example.com"},
		{"#section-1", "#section-1"},
		{"article.html", "article.html"},
		{"index.html", "index.html"},
		{"./article.html", "article.html"},
		{"../README.md", "https://github.com/darkliquid/ironwail-go/blob/main/README.md"},
		{"assets/screenshot.png", "https://github.com/darkliquid/ironwail-go/blob/main/article/assets/screenshot.png"},
	}

	for _, tt := range tests {
		got := remapRepoLink(tt.input)
		if got != tt.want {
			t.Errorf("remapRepoLink(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
