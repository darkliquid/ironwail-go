package renderer

import oitimpl "github.com/ironwail/ironwail-go/internal/renderer/oit"

// shouldSortTranslucentCalls reports whether translucent draw calls should be
// depth-sorted before rendering.
func shouldSortTranslucentCalls(mode AlphaMode) bool {
	return oitimpl.ShouldSortTranslucentCalls(int(mode))
}
