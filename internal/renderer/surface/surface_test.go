package surface

import (
	"testing"
)

func TestBuildTextureAnimations(t *testing.T) {
	names := []string{
		"base_wall",
		"+0button",
		"+1button",
		"+2button",
	}

	textures, err := BuildTextureAnimations(names)
	if err != nil {
		t.Fatalf("BuildTextureAnimations failed: %v", err)
	}

	if len(textures) != len(names) {
		t.Fatalf("len(textures) = %d, want %d", len(textures), len(names))
	}

	if textures[0] == nil {
		t.Fatal("textures[0] should not be nil")
	}
	if textures[0].AnimNext != nil {
		t.Error("non-animated texture should have nil AnimNext")
	}

	btn0 := textures[1]
	if btn0 == nil {
		t.Fatal("textures[1] (+0button) should not be nil")
	}
	if btn0.AnimTotal <= 0 {
		t.Errorf("btn0.AnimTotal = %d, want > 0", btn0.AnimTotal)
	}

	if btn0.AnimNext == nil {
		t.Fatal("btn0.AnimNext should not be nil")
	}
}
