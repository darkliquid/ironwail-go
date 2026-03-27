package oit

const AlphaModeOIT = 2

// ShouldUseResources reports whether weighted blended OIT framebuffer
// resources should be active for the current alpha compositing mode.
func ShouldUseResources(alphaMode int) bool {
	return alphaMode == AlphaModeOIT
}

// ShouldSortTranslucentCalls reports whether translucent draw calls should be
// depth-sorted before rendering.
func ShouldSortTranslucentCalls(alphaMode int) bool {
	return alphaMode != AlphaModeOIT
}
