package renderer

// ShouldUseOITResources reports whether weighted blended OIT framebuffer
// resources should be active for the current alpha compositing mode.
func ShouldUseOITResources() bool {
	return GetAlphaMode() == AlphaModeOIT
}
