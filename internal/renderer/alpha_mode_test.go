package renderer

import (
	"testing"
)

func ensureAlphaModeCvars() {
	testCV.Register(CvarROIT, "0", 0, "OIT test cvar")
	testCV.Register(CvarRAlphaSort, "0", 0, "Alpha sort test cvar")
}

func TestGetAlphaMode(t *testing.T) {
	ensureAlphaModeCvars()

	t.Run("oit takes precedence", func(t *testing.T) {
		testCV.SetBool(CvarROIT, true)
		testCV.SetBool(CvarRAlphaSort, false)
		if got := GetAlphaMode(); got != AlphaModeOIT {
			t.Fatalf("GetAlphaMode() = %v, want %v", got, AlphaModeOIT)
		}
	})

	t.Run("sorted when oit disabled", func(t *testing.T) {
		testCV.SetBool(CvarROIT, false)
		testCV.SetBool(CvarRAlphaSort, true)
		if got := GetAlphaMode(); got != AlphaModeSorted {
			t.Fatalf("GetAlphaMode() = %v, want %v", got, AlphaModeSorted)
		}
	})

	t.Run("basic when both disabled", func(t *testing.T) {
		testCV.SetBool(CvarROIT, false)
		testCV.SetBool(CvarRAlphaSort, false)
		if got := GetAlphaMode(); got != AlphaModeBasic {
			t.Fatalf("GetAlphaMode() = %v, want %v", got, AlphaModeBasic)
		}
	})
}

func TestSetAlphaMode(t *testing.T) {
	ensureAlphaModeCvars()

	t.Run("basic mode", func(t *testing.T) {
		testCV.SetBool(CvarROIT, true)
		testCV.SetBool(CvarRAlphaSort, true)
		SetAlphaMode(AlphaModeBasic)
		if testCV.BoolValue(CvarROIT) {
			t.Fatal("r_oit = 1, want 0")
		}
		if testCV.BoolValue(CvarRAlphaSort) {
			t.Fatal("r_alphasort = 1, want 0")
		}
	})

	t.Run("sorted mode", func(t *testing.T) {
		testCV.SetBool(CvarROIT, true)
		testCV.SetBool(CvarRAlphaSort, false)
		SetAlphaMode(AlphaModeSorted)
		if testCV.BoolValue(CvarROIT) {
			t.Fatal("r_oit = 1, want 0")
		}
		if !testCV.BoolValue(CvarRAlphaSort) {
			t.Fatal("r_alphasort = 0, want 1")
		}
	})

	t.Run("oit mode preserves alphasort fallback", func(t *testing.T) {
		testCV.SetBool(CvarRAlphaSort, true)
		testCV.SetBool(CvarROIT, false)
		SetAlphaMode(AlphaModeOIT)
		if !testCV.BoolValue(CvarROIT) {
			t.Fatal("r_oit = 0, want 1")
		}
		if !testCV.BoolValue(CvarRAlphaSort) {
			t.Fatal("r_alphasort changed unexpectedly while enabling OIT")
		}
	})
}

func TestAlphaModeString(t *testing.T) {
	tests := []struct {
		mode AlphaMode
		want string
	}{
		{mode: AlphaModeBasic, want: "BASIC"},
		{mode: AlphaModeSorted, want: "SORTED"},
		{mode: AlphaModeOIT, want: "OIT"},
		{mode: AlphaMode(999), want: "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Fatalf("AlphaMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}
