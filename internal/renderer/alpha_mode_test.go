package renderer

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/cvar"
)

func ensureAlphaModeCvars() {
	cvar.Register(CvarROIT, "0", 0, "OIT test cvar")
	cvar.Register(CvarRAlphaSort, "0", 0, "Alpha sort test cvar")
}

func TestGetAlphaMode(t *testing.T) {
	ensureAlphaModeCvars()

	t.Run("oit takes precedence", func(t *testing.T) {
		cvar.SetBool(CvarROIT, true)
		cvar.SetBool(CvarRAlphaSort, false)
		if got := GetAlphaMode(); got != AlphaModeOIT {
			t.Fatalf("GetAlphaMode() = %v, want %v", got, AlphaModeOIT)
		}
	})

	t.Run("sorted when oit disabled", func(t *testing.T) {
		cvar.SetBool(CvarROIT, false)
		cvar.SetBool(CvarRAlphaSort, true)
		if got := GetAlphaMode(); got != AlphaModeSorted {
			t.Fatalf("GetAlphaMode() = %v, want %v", got, AlphaModeSorted)
		}
	})

	t.Run("basic when both disabled", func(t *testing.T) {
		cvar.SetBool(CvarROIT, false)
		cvar.SetBool(CvarRAlphaSort, false)
		if got := GetAlphaMode(); got != AlphaModeBasic {
			t.Fatalf("GetAlphaMode() = %v, want %v", got, AlphaModeBasic)
		}
	})
}

func TestSetAlphaMode(t *testing.T) {
	ensureAlphaModeCvars()

	t.Run("basic mode", func(t *testing.T) {
		cvar.SetBool(CvarROIT, true)
		cvar.SetBool(CvarRAlphaSort, true)
		SetAlphaMode(AlphaModeBasic)
		if cvar.BoolValue(CvarROIT) {
			t.Fatal("r_oit = 1, want 0")
		}
		if cvar.BoolValue(CvarRAlphaSort) {
			t.Fatal("r_alphasort = 1, want 0")
		}
	})

	t.Run("sorted mode", func(t *testing.T) {
		cvar.SetBool(CvarROIT, true)
		cvar.SetBool(CvarRAlphaSort, false)
		SetAlphaMode(AlphaModeSorted)
		if cvar.BoolValue(CvarROIT) {
			t.Fatal("r_oit = 1, want 0")
		}
		if !cvar.BoolValue(CvarRAlphaSort) {
			t.Fatal("r_alphasort = 0, want 1")
		}
	})

	t.Run("oit mode preserves alphasort fallback", func(t *testing.T) {
		cvar.SetBool(CvarRAlphaSort, true)
		cvar.SetBool(CvarROIT, false)
		SetAlphaMode(AlphaModeOIT)
		if !cvar.BoolValue(CvarROIT) {
			t.Fatal("r_oit = 0, want 1")
		}
		if !cvar.BoolValue(CvarRAlphaSort) {
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
