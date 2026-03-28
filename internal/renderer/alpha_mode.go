package renderer

import "github.com/darkliquid/ironwail-go/internal/cvar"

// AlphaMode selects how translucent surfaces are composited.
// This mirrors C Ironwail's alphamode_t behavior:
// OIT has priority over sorted/basic, and sorted/basic are selected by r_alphasort.
type AlphaMode int

const (
	AlphaModeBasic AlphaMode = iota
	AlphaModeSorted
	AlphaModeOIT
)

// String returns a stable debug name for the current alpha mode.
func (m AlphaMode) String() string {
	switch m {
	case AlphaModeBasic:
		return "BASIC"
	case AlphaModeSorted:
		return "SORTED"
	case AlphaModeOIT:
		return "OIT"
	default:
		return "UNKNOWN"
	}
}

// GetAlphaMode resolves the active transparency mode from cvars.
// r_oit takes precedence over r_alphasort, matching C Ironwail.
func GetAlphaMode() AlphaMode {
	if cvar.BoolValue(CvarROIT) {
		return AlphaModeOIT
	}
	if cvar.BoolValue(CvarRAlphaSort) {
		return AlphaModeSorted
	}
	return AlphaModeBasic
}

// SetAlphaMode updates cvars to represent the requested transparency mode.
// For OIT, r_alphasort is intentionally left unchanged so previous fallback
// preference is preserved if OIT is disabled later.
func SetAlphaMode(mode AlphaMode) {
	cvar.SetBool(CvarROIT, mode == AlphaModeOIT)
	if mode != AlphaModeOIT {
		cvar.SetBool(CvarRAlphaSort, mode == AlphaModeSorted)
	}
}
