package testutil

import (
	"bufio"
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
			return werr
		}
		name := d.Name()
		if d.IsDir() {
			switch name {
			case ".git", "vendor", "node_modules", ".yggdrasil":
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
		n, err := countLines(path)
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

func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	n := 0
	for sc.Scan() {
		n++
	}
	return n, sc.Err()
}

func isGenerated(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
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
