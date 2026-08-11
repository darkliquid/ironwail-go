//go:build js && wasm

package main

import (
	"syscall/js"
	"testing"
)

// TestToJSValueShapesConvertible ensures every shape the inspector returns is
// convertible by syscall/js.ValueOf (which panics on []float32, []string,
// []map[string]any, [32]int). This guards the browser "ValueOf: invalid
// value" crash class.
func TestToJSValueShapesConvertible(t *testing.T) {
	inputs := []any{
		map[string]any{
			"lines": []string{"a", "b"},
			"origin": []float32{1, 2, 3},
			"edicts": []map[string]any{
				{"num": 1, "free": false, "origin": []float32{0, 0, 0}},
			},
			"stats": [32]int{1, 2, 3},
			"bytes": []int{4, 5},
		},
		[]float32{1, 2},
		[]map[string]any{{"a": []string{"x"}}},
		"plain",
		42,
		3.5,
		true,
		nil,
	}
	for _, in := range inputs {
		out := toJSValue(in)
		// ValueOf must not panic for the converted shape.
		_ = js.ValueOf(out)
	}
}
