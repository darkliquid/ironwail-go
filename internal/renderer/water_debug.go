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
