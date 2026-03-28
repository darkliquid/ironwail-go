package renderer

import oitimpl "github.com/darkliquid/ironwail-go/internal/renderer/oit"

// ShouldUseOITResources reports whether weighted blended OIT framebuffer
// resources should be active for the current alpha compositing mode.
func ShouldUseOITResources() bool {
	return oitimpl.ShouldUseResources(int(GetAlphaMode()))
}
