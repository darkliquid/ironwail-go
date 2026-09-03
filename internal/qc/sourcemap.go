package qc

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"sync"
)

// SourceMapVersion is the schema version of the side-car source map file.
const SourceMapVersion = 1

// SourceMap maps compiled progs.dat statement indices back to the QuakeGo
// source positions they were lowered from — the bytecode analogue of the
// JavaScript source maps that map minified output to original TypeScript.
//
// It is a side-car file (by convention `<output>.map` next to progs.dat) and
// never changes the progs.dat binary format itself.
//
// The JSON form uses explicit per-statement mappings rather than the JS
// spec's VLQ encoding: QCVM "generated code" is bytecode statements rather
// than text lines, so there is no line/column continuum to delta-encode, and
// the explicit form is directly consumable by the DAP without a decoder.
//
// Like a JS source map, Sources are stored relative to SourceRoot (the mod
// directory the compiler ran against) so the map stays portable; a consumer
// joins SourceRoot with a relative source to reach the original file.
type SourceMap struct {
	Version    int      `json:"version"`
	File       string   `json:"file"`       // generated artifact (e.g. "progs.dat")
	SourceRoot string   `json:"sourceRoot"` // prefix for relative Sources
	Sources    []string `json:"sources"`    // original QuakeGo source files

	// Mappings is sorted by Stmt ascending. Consecutive statements lowered
	// from the same source line each get an entry; statements with no origin
	// (sentinels, prologues) have Source < 0 and are not breakpointable.
	Mappings []SourceMapping `json:"mappings"`

	indexOnce sync.Once
	byLine    map[int]map[int][]int // source index -> line -> sorted stmts
}

// SourceMapping maps one progs statement to its QuakeGo origin.
type SourceMapping struct {
	Stmt   int    `json:"stmt"`   // progs.dat statement index
	Source int    `json:"source"` // index into Sources (-1 = no origin)
	Line   int    `json:"line"`   // 1-based line in the source file
	Col    int    `json:"col"`    // 1-based column (0 = unknown)
	Func   string `json:"func"`   // enclosing QC function name
}

// sourceMapFileJSON is the on-disk JSON shape.
type sourceMapFileJSON struct {
	Version    int             `json:"version"`
	File       string          `json:"file"`
	SourceRoot string          `json:"sourceRoot"`
	Sources    []string        `json:"sources"`
	Mappings   []SourceMapping `json:"mappings"`
}

// Write serializes the source map as JSON.
func (sm *SourceMap) Write(w io.Writer) error {
	if sm == nil {
		return fmt.Errorf("nil source map")
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(sourceMapFileJSON{
		Version:    sm.Version,
		File:       sm.File,
		SourceRoot: sm.SourceRoot,
		Sources:    sm.Sources,
		Mappings:   sm.Mappings,
	})
}

// LoadSourceMap parses a source map previously written by Write.
func LoadSourceMap(r io.Reader) (*SourceMap, error) {
	var raw sourceMapFileJSON
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode source map: %w", err)
	}
	if raw.Version != SourceMapVersion {
		return nil, fmt.Errorf("source map version %d, want %d", raw.Version, SourceMapVersion)
	}
	sm := &SourceMap{
		Version:    raw.Version,
		File:       raw.File,
		SourceRoot: raw.SourceRoot,
		Sources:    raw.Sources,
		Mappings:   raw.Mappings,
	}
	// Binary-search Lookup requires ascending Stmt order; sort defensively so
	// hand-edited or third-party maps behave identically to compiler output.
	sort.SliceStable(sm.Mappings, func(a, b int) bool {
		return sm.Mappings[a].Stmt < sm.Mappings[b].Stmt
	})
	return sm, nil
}

// Lookup returns the mapping for a statement index, or nil when the statement
// is outside the mapped range. O(log n).
func (sm *SourceMap) Lookup(stmt int) *SourceMapping {
	if sm == nil {
		return nil
	}
	lo, hi := 0, len(sm.Mappings)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		switch {
		case sm.Mappings[mid].Stmt < stmt:
			lo = mid + 1
		case sm.Mappings[mid].Stmt > stmt:
			hi = mid - 1
		default:
			return &sm.Mappings[mid]
		}
	}
	return nil
}

// StatementsForLine returns every statement index originating from the given
// source file and 1-based line — what a debugger needs to resolve a source
// line breakpoint onto bytecode statements. Sorted ascending.
//
// file matches an entry of Sources (relative to SourceRoot, as written);
// callers holding an absolute path should use SourceFileKey to normalize.
// SourceIndexForFile resolves a file path (as sent by a DAP client: absolute,
// workspace-relative, or bare) to its index in Sources. Matching tries the
// exact relative path first, then the base name — clients rarely send paths
// in the map's mod-dir-relative form. Returns -1 when not found.
func (sm *SourceMap) SourceIndexForFile(file string) int {
	sm.ensureIndex()
	for i, s := range sm.Sources {
		if s == file {
			return i
		}
	}
	base := filepath.Base(file)
	for i, s := range sm.Sources {
		if filepath.Base(s) == base {
			return i
		}
	}
	return -1
}

// StatementsForLine returns every statement index originating from the given
// source file and 1-based line — what a debugger needs to resolve a source
// line breakpoint onto bytecode statements. Sorted ascending. file matches a
// Sources entry (see SourceIndexForFile for accepted forms).
func (sm *SourceMap) StatementsForLine(file string, line int) []int {
	src := sm.SourceIndexForFile(file)
	if src < 0 {
		return nil
	}
	return sm.StatementsForLineByIndex(src, line)
}

// StatementsForLineByIndex is StatementsForLine keyed by a Sources index,
// letting callers resolve the index once and reuse it across lines.
func (sm *SourceMap) StatementsForLineByIndex(src, line int) []int {
	sm.ensureIndex()
	if src < 0 {
		return nil
	}
	stmts := make([]int, 0, len(sm.byLine[src][line]))
	stmts = append(stmts, sm.byLine[src][line]...)
	return stmts
}

// ResolveSource returns the on-disk path for a source entry: SourceRoot
// joined with the relative source when a root is set, else the entry as-is.
// The idx must be a valid index into Sources; returns "" otherwise.
func (sm *SourceMap) ResolveSource(idx int) string {
	if sm == nil || idx < 0 || idx >= len(sm.Sources) {
		return ""
	}
	if sm.SourceRoot == "" {
		return sm.Sources[idx]
	}
	return filepath.Join(sm.SourceRoot, sm.Sources[idx])
}

// SourceFileKey normalizes a path for matching against Sources entries: the
// base name, since a DAP client may send absolute or workspace-relative paths
// while the map stores mod-dir-relative ones.
func SourceFileKey(path string) string {
	return filepath.Base(path)
}

// SourceRelName returns the Sources entry as written (relative to
// SourceRoot), suitable for display in a debugger UI. idx must be a valid
// index into Sources; returns "" otherwise.
func (sm *SourceMap) SourceRelName(idx int) string {
	if sm == nil || idx < 0 || idx >= len(sm.Sources) {
		return ""
	}
	return sm.Sources[idx]
}

// ensureIndex builds the line index once. Safe for concurrent use.
func (sm *SourceMap) ensureIndex() {
	sm.indexOnce.Do(func() {
		sm.byLine = make(map[int]map[int][]int)
		for _, m := range sm.Mappings {
			if m.Source < 0 {
				continue
			}
			if sm.byLine[m.Source] == nil {
				sm.byLine[m.Source] = make(map[int][]int)
			}
			sm.byLine[m.Source][m.Line] = append(sm.byLine[m.Source][m.Line], m.Stmt)
		}
		for src := range sm.byLine {
			for line := range sm.byLine[src] {
				stmts := sm.byLine[src][line]
				sort.Ints(stmts)
				sm.byLine[src][line] = stmts
			}
		}
	})
}
