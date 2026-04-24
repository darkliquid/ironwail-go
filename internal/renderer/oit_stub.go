package renderer

// oitFramebuffers keeps the shared OIT zero-value shape available even when
// no renderer backend provides concrete OIT resources.
type oitFramebuffers struct {
	fbo     uint32
	samples int
}
