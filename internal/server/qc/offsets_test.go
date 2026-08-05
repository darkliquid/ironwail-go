package qc

import "testing"

func TestDefaultEntFieldOffsets(t *testing.T) {
	offsets := DefaultEntFieldOffsets()
	if len(offsets) == 0 {
		t.Fatal("expected non-empty entvar field offset table")
	}
	// Spot-check well-known fields resolve to distinct non-zero offsets.
	known := []string{"Origin", "Velocity", "Angles", "Health", "ClassName", "Model"}
	for _, name := range known {
		if _, ok := offsets[NormalizeFieldName(name)]; !ok {
			t.Errorf("DefaultEntFieldOffsets missing field %q", name)
		}
	}
}

func TestNormalizeFieldName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"ClassName", "classname"},
		{"class_name", "classname"},
		{"classname", "classname"},
		{"TargetName", "targetname"},
		{"WeaponModel", "weaponmodel"},
	}
	for _, c := range cases {
		if got := NormalizeFieldName(c.in); got != c.want {
			t.Errorf("NormalizeFieldName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
