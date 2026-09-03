// Package transpile converts QuakeC source into QuakeGo (Go-syntax) source.
//
// It is a migration aid, not a semantics-preserving compiler: the output
// targets the conventions of pkg/qgo/quakego (exported engine globals,
// PascalCase entity fields, engine.X builtin calls) and emits explicit
// TODO(transpile) comments for constructs that need human judgment
// (function-pointer field reassignments, QC compiler extensions). Run it,
// refine the result, and keep the QuakeGo source under version control.
package transpile

import "fmt"

// Options controls transpilation.
type Options struct {
	// Package is the Go package clause for the output (default "progs").
	Package string
}

// Transpile converts QuakeC source to QuakeGo source text.
func Transpile(src string, opts Options) (string, error) {
	decls, err := Parse(src)
	if err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}
	e := &Emitter{Package: opts.Package}
	e.EmitProgram(decls)
	return e.Output(), nil
}
