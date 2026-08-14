package renderer

import (
	"testing"
)

func TestRenderPassFlags(t *testing.T) {
	// Default is all enabled
	SetGlobalPassFlags(PassAll)
	if !IsGlobalPassEnabled(PassSky) || !IsGlobalPassEnabled(PassWorldOpaque) {
		t.Fatalf("Expected all passes to be enabled by default")
	}

	// Disable sky
	SetGlobalPassEnabled(PassSky, false)
	if IsGlobalPassEnabled(PassSky) {
		t.Errorf("Expected PassSky to be disabled")
	}
	if !IsGlobalPassEnabled(PassWorldOpaque) {
		t.Errorf("Expected PassWorldOpaque to remain enabled")
	}

	// Enable sky back
	SetGlobalPassEnabled(PassSky, true)
	if !IsGlobalPassEnabled(PassSky) {
		t.Errorf("Expected PassSky to be enabled")
	}

	// Test string toggle API
	toggles := GetPassTogglesMap()
	if !toggles["sky"] || !toggles["world"] || !toggles["overlay"] {
		t.Errorf("Expected toggles to be true: %+v", toggles)
	}

	if !SetPassToggleByName("lightmaps", false) {
		t.Errorf("SetPassToggleByName failed for 'lightmaps'")
	}
	if IsGlobalPassEnabled(PassLightmaps) {
		t.Errorf("Expected PassLightmaps to be disabled")
	}

	if !SetPassToggleByName("lightmaps", true) {
		t.Errorf("SetPassToggleByName failed for 'lightmaps'")
	}
	if !IsGlobalPassEnabled(PassLightmaps) {
		t.Errorf("Expected PassLightmaps to be enabled")
	}

	// Invalid pass name
	if SetPassToggleByName("nonexistent", false) {
		t.Errorf("Expected SetPassToggleByName to return false for invalid pass")
	}

	// Reset
	SetGlobalPassFlags(PassAll)
}
