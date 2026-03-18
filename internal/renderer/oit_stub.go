//go:build !((opengl || cgo) && !gogpu)

package renderer

// oitFramebuffers mirrors the OpenGL OIT resource layout for non-OpenGL builds
// so shared logic and tests can still reason about initialization state.
type oitFramebuffers struct {
	accumTex     uint32
	revealageTex uint32
	fbo          uint32
	width        int
	height       int
	samples      int
}
