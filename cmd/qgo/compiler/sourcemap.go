package compiler

import (
	"path/filepath"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

// BuildSourceMap assembles a source map from compiler output, mapping progs
// statement indices to the QuakeGo source positions they were lowered from.
// The map is a side-car artifact — the progs.dat binary is unchanged.
//
// file is the name of the generated artifact (for the JSON "file" field).
// sourceRoot is the directory the Sources entries are recorded relative to
// (empty keeps them as-is). Positions whose filename is empty or invalid get
// a mapping with Source < 0: they have no QuakeGo origin (sentinels,
// prologues) and are not breakpointable.
func BuildSourceMap(in *EmitInput, file, sourceRoot string) *qc.SourceMap {
	sm := &qc.SourceMap{
		Version:    qc.SourceMapVersion,
		File:       file,
		SourceRoot: sourceRoot,
		Mappings:   make([]qc.SourceMapping, 0, len(in.Statements)),
	}
	sources := map[string]int{}

	rel := func(p string) string {
		if sourceRoot == "" {
			return p
		}
		if relPath, err := filepath.Rel(sourceRoot, p); err == nil {
			return relPath
		}
		return p
	}

	for i, pos := range in.SourcePos {
		m := qc.SourceMapping{Stmt: i, Source: -1}
		m.Func = fnNameForStmt(in, i)
		if pos.IsValid() && pos.Filename != "" {
			display := rel(pos.Filename)
			src, ok := sources[display]
			if !ok {
				src = len(sm.Sources)
				sources[display] = src
				sm.Sources = append(sm.Sources, display)
			}
			m.Source, m.Line, m.Col = src, pos.Line, pos.Column
		}
		sm.Mappings = append(sm.Mappings, m)
	}
	return sm
}

// fnNameForStmt finds the QC function that owns a statement index: the
// bytecode function with the greatest FirstStatement <= stmt. Builtins own no
// statements. Returns "" when no function claims the statement.
func fnNameForStmt(in *EmitInput, stmt int) string {
	best := -1
	for fi := range in.Functions {
		f := &in.Functions[fi]
		if f.FirstStatement < 0 || int(f.FirstStatement) > stmt {
			continue
		}
		if best < 0 || f.FirstStatement > in.Functions[best].FirstStatement {
			best = fi
		}
	}
	if best < 0 {
		return ""
	}
	return emitInputString(in, in.Functions[best].Name)
}

// emitInputString extracts a NUL-terminated string from the progs string table.
func emitInputString(in *EmitInput, ofs int32) string {
	if ofs < 0 || int(ofs) >= len(in.Strings) {
		return ""
	}
	end := ofs
	for int(end) < len(in.Strings) && in.Strings[end] != 0 {
		end++
	}
	return string(in.Strings[ofs:end])
}
