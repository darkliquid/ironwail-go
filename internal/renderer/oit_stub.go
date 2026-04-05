package renderer

// oitFramebuffers keeps the shared OIT zero-value shape available even when
// no renderer backend provides concrete OIT resources.
type oitFramebuffers struct {
	accumTex     uint32
	revealageTex uint32
	fbo          uint32
	width        int
	height       int
	samples      int
}
