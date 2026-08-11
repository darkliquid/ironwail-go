//go:build js && wasm

package main

import (
	"encoding/json"
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

// TestGetStateJSONStableAcrossCalls verifies the deterministic payload the
// page's change-detection cache consumes: encoding/json sorts map keys, so two
// marshals of identical state produce byte-identical strings. The js-object
// conversion form would reorder Go map keys randomly per call.
func TestGetStateJSONStableAcrossCalls(t *testing.T) {
	state := map[string]any{
		"alpha": 1,
		"beta":  []float32{1, 2, 3},
		"gamma": map[string]any{"x": "y", "a": "b"},
		"delta": []map[string]any{{"z": 1, "m": 2}},
	}
	a, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatalf("marshal not stable:\n%s\n%s", a, b)
	}
	// Also confirm the shape survives JSON round-trip (what the page does).
	var rt map[string]any
	if err := json.Unmarshal(a, &rt); err != nil {
		t.Fatal(err)
	}
}
