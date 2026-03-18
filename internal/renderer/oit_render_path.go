package renderer

// shouldSortTranslucentCalls reports whether translucent draw calls should be
// depth-sorted before rendering.
func shouldSortTranslucentCalls(mode AlphaMode) bool {
	return mode != AlphaModeOIT
}
