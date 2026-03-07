package input

import "testing"

func TestFunctionKeyStringRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		key  int
		name string
	}{
		{key: KF1, name: "F1"},
		{key: KF9, name: "F9"},
		{key: KF10, name: "F10"},
		{key: KF11, name: "F11"},
		{key: KF12, name: "F12"},
	} {
		if got := KeyToString(tc.key); got != tc.name {
			t.Fatalf("KeyToString(%d) = %q, want %q", tc.key, got, tc.name)
		}
		if got := StringToKey(tc.name); got != tc.key {
			t.Fatalf("StringToKey(%q) = %d, want %d", tc.name, got, tc.key)
		}
	}
}
