package renderer

// rDebugWaterEnabled returns true if the r_debug_water cvar is non-zero.
func rDebugWaterEnabled() bool {
	if pkgCVars == nil {
		return false
	}
	cv := pkgCVars.Get(CvarRDebugWater)
	if cv == nil {
		return false
	}
	return cv.Int != 0
}

// rDebugOITEnabled returns true if either the r_debug_oit or r_debug_water cvar is non-zero.
func rDebugOITEnabled() bool {
	if pkgCVars == nil {
		return false
	}
	if cv := pkgCVars.Get(CvarRDebugOIT); cv != nil && cv.Int != 0 {
		return true
	}
	return rDebugWaterEnabled()
}
