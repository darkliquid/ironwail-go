package testutil

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// maxGoFileLines is the project-wide ceiling on `.go` source file length
// (including _test.go files). Files approaching this limit should be split
// into thematic companions to keep navigation and review manageable.
const maxGoFileLines = 1000

// knownOversizedFiles lists Go files that already exceed maxGoFileLines at
// the time this guard was introduced. The entries here are a shrinking
// debt list: the test fails if a file in this allowlist drops at or below
// the ceiling (remove it from the list) OR if a brand-new file exceeds the
// ceiling. This prevents regressions while allowing the existing debt to
// be paid down one file at a time.
//
// IMPORTANT: When you split a file below the ceiling, DELETE its entry
// from this map. When all entries are gone the guard becomes a hard
// ceiling over the whole tree.
var knownOversizedFiles = map[string]struct{}{}

// TestProjectFilesUnderLineCeiling walks the repository root and fails if
// any `.go` file exceeds maxGoFileLines, unless the file is listed in
// knownOversizedFiles. Vendored, generated, and .git paths are skipped.
//
// The test also fails if a file listed in knownOversizedFiles is now under
// the ceiling (debt paid — remove the entry).
func TestProjectFilesUnderLineCeiling(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	var offenders []string
	var paidDown []string
	seenAllowlisted := map[string]bool{}
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, werr error) error {
		if werr != nil {
			// Tolerate races with transient build cache directories
			// (e.g. under .tmp where `go test` creates and destroys
			// go-build*** scratch dirs mid-walk). We skip any path
			// that disappeared between Readdir and our visit.
			if errors.Is(werr, os.ErrNotExist) {
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			return werr
		}
		name := d.Name()
		if d.IsDir() {
			switch name {
			case ".git", "vendor", "node_modules", ".yggdrasil", ".tmp":
				return filepath.SkipDir
			}
			if strings.HasPrefix(name, "go-build") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		if isGenerated(path) {
			return nil
		}
		n, err := countCodeLines(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		_, allowed := knownOversizedFiles[rel]
		if allowed {
			seenAllowlisted[rel] = true
			if n <= maxGoFileLines {
				paidDown = append(paidDown, rel)
			}
			return nil
		}
		if n > maxGoFileLines {
			offenders = append(offenders, formatOffender(rel, n))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(offenders) > 0 {
		t.Errorf("the following Go files exceed %d lines and must be split:\n  %s",
			maxGoFileLines, strings.Join(offenders, "\n  "))
	}
	if len(paidDown) > 0 {
		t.Errorf("the following files are now under %d lines; remove them from knownOversizedFiles in file_length_test.go:\n  %s",
			maxGoFileLines, strings.Join(paidDown, "\n  "))
	}
	for rel := range knownOversizedFiles {
		if !seenAllowlisted[rel] {
			t.Errorf("knownOversizedFiles entry %q no longer exists; remove it", rel)
		}
	}
}

// countCodeLines returns the number of actual code lines in a Go file:
// blank lines and comment-only lines (both // and /* */, including
// multi-line block comments) are excluded. The ceiling is meant to keep
// navigation and review manageable, so whitespace and prose should not
// count against it.
func countCodeLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	n := 0
	inBlockComment := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if inBlockComment {
			// Still inside a /* ... */ block; ends when */ appears.
			if idx := strings.Index(line, "*/"); idx >= 0 {
				inBlockComment = false
				rest := strings.TrimSpace(line[idx+2:])
				if rest == "" {
					// Only comment content on this line.
					continue
				}
				// Code follows the block comment close; count it.
				line = rest
			} else {
				continue
			}
		}
		// Strip a leading // line comment (everything after it on this
		// line is prose).
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
			if line == "" {
				continue
			}
		}
		// Handle /* ... */ on a single (or continuing) line.
		if idx := strings.Index(line, "/*"); idx >= 0 {
			comment := line[idx:]
			line = strings.TrimSpace(line[:idx])
			if strings.Contains(comment, "*/") {
				// Same-line block comment close; skip it, count code
				// before the comment.
				if line != "" {
					n++
				}
				continue
			}
			inBlockComment = true
			if line != "" {
				n++
			}
			continue
		}
		if line != "" {
			n++
		}
	}
	return n, sc.Err()
}

func isGenerated(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for i := 0; i < 5 && sc.Scan(); i++ {
		line := sc.Text()
		if strings.Contains(line, "DO NOT EDIT") || strings.Contains(line, "Code generated") {
			return true
		}
	}
	return false
}

func formatOffender(rel string, n int) string {
	return rel + " (" + itoa(n) + " lines)"
}

// itoa avoids pulling strconv for a single call site.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func TestCountCodeLinesSkipsBlankAndComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.go")
	content := `package sample

// doc comment
/*
block
comment
*/

// leading comment then code
var x = 1
`
	// 2 code lines: "package sample" and "var x = 1"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	n, err := countCodeLines(path)
	if err != nil {
		t.Fatalf("countCodeLines: %v", err)
	}
	if n != 2 {
		t.Fatalf("code lines = %d, want 2", n)
	}
}

func TestCountCodeLinesBlockInline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.go")
	content := "package x\n\n/* c */ var a = 1\nvar b = 2 // tail\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	n, err := countCodeLines(path)
	if err != nil {
		t.Fatalf("countCodeLines: %v", err)
	}
	// "/* c */ var a = 1" and "var b = 2" are code lines.
	if n != 2 {
		t.Fatalf("code lines = %d, want 2", n)
	}
}
