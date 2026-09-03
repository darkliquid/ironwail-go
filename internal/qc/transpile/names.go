package transpile

import (
	"fmt"
	"strings"
)

// goTypeName maps a QC type to its QuakeGo type. The empty string means
// "no return value" (QC void).
func goTypeName(qcType string) string {
	switch qcType {
	case "void":
		return ""
	case "float":
		return "float32"
	case "string":
		return "string"
	case "vector":
		return "quake.Vec3"
	case "entity":
		return "*quake.Entity"
	}
	return qcType
}

// engineGlobals are the QC engine globals the QuakeGo port exposes as
// package-level vars in the quake module (self, other, time, ...).
var engineGlobals = map[string]string{
	"self":             "Self",
	"other":            "Other",
	"world":            "World",
	"time":             "Time",
	"activator":        "Activator",
	"msg_entity":       "MsgEntity",
	"msg_entity ":      "MsgEntity",
	"return":           "Return",
	"trace_allsolid":   "TraceAllSolid",
	"trace_startsolid": "TraceStartSolid",
}

// goName maps a QC global/constant name to its QuakeGo form. Engine globals
// use their exported names; ALL_CAPS constants keep their QC spelling (the
// QuakeGo port preserves MOVETYPE_NONE-style constant names); other globals
// get snake_case -> PascalCase (found_secrets -> FoundSecrets).
func goName(qcName string) string {
	if g, ok := engineGlobals[strings.ToLower(qcName)]; ok {
		return g
	}
	if p, ok := builtinNames[strings.ToLower(qcName)]; ok {
		return p
	}
	if isAllCaps(qcName) {
		return qcName
	}
	return pascalCase(qcName)
}

// isAllCaps reports whether an identifier is an ALL_CAPS constant: only
// uppercase letters, digits, and underscores, with at least one letter.
func isAllCaps(s string) bool {
	hasLetter := false
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			return false
		}
		if (r >= 'A' && r <= 'Z') || r == '_' || (r >= '0' && r <= '9') {
			if r >= 'A' && r <= 'Z' {
				hasLetter = true
			}
			continue
		}
		return false
	}
	return hasLetter
}

// qcFuncName maps a QC function name to its QuakeGo form: the name is kept
// as-is (the QuakeGo port preserves trigger_reactivate-style names).
func qcFuncName(qcName string) string { return qcName }

// fieldName maps a QC entity field name to its QuakeGo struct field.
// The Entity struct's qgo tags define the canonical mapping (see
// entityfields.go); mod-specific fields fall back to pascalCase.
func fieldName(qcName string) string {
	if f, ok := entityFields[qcName]; ok {
		return f
	}
	return pascalCase(qcName)
}

// pascalCase converts snake_case (or flat lowercase) to PascalCase.
func pascalCase(s string) string {
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		r := []rune(part)
		b.WriteRune(upper(r[0]))
		b.WriteString(string(r[1:]))
	}
	return b.String()
}

// upper uppercases a single rune if it is an ASCII lowercase letter.
func upper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 'a' + 'A'
	}
	return r
}

// builtinNames maps QC builtin/built-in-adjacent names to their QuakeGo
// call form. Names not present fall back to engine.<PascalCase>, which
// matches the engine package's exported function names for the majority of
// builtins. QC statement builtins that are not engine calls (break,
// continue) are handled at the statement level, not here.
var builtinNames = map[string]string{
	"true":        "True",
	"false":       "False",
	"string_null": "StringNull",
}

// builtinCall renders a call to a QC builtin in QuakeGo form. Names found in
// the engineBuiltins table become engine.<GoName> calls; anything else is a
// user function (or a QC-level function) and keeps its source spelling so
// calls resolve within the transpiled module.
func builtinCall(qcName string, args []string) string {
	lower := strings.ToLower(qcName)

	// QC statement builtins that need special Go forms.
	switch lower {
	case "break":
		return "panic(engine.BreakStatement())"
	}

	if goFn, ok := engineBuiltins[lower]; ok {
		return fmt.Sprintf("engine.%s(%s)", goFn, strings.Join(args, ", "))
	}

	// User function or QC-layer function: keep the source spelling.
	return fmt.Sprintf("%s(%s)", qcName, strings.Join(args, ", "))
}
