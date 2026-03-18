//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"fmt"

	"github.com/go-gl/gl/v4.6-core/gl"
)

// oitFramebuffers holds GPU resources for weighted blended OIT.
// Two textures form a Multiple Render Target (MRT) setup:
//   - accumTex (RGBA16F): weighted color accumulation
//   - revealageTex (R8): per-pixel alpha coverage
//
// The FBO attaches both to COLOR_ATTACHMENT0 and COLOR_ATTACHMENT1.
type oitFramebuffers struct {
	accumTex     uint32 // GL_RGBA16F accumulation texture
	revealageTex uint32 // GL_R8 revealage texture
	fbo          uint32 // FBO with both textures as MRT
	width        int
	height       int
	samples      int // MSAA sample count (0/1 = single-sample textures)
}

// ensureOITFramebuffers creates/recreates weighted blended OIT render targets.
// OIT uses MRT output:
//   - attachment 0: accumulation buffer (RGBA16F)
//   - attachment 1: revealage buffer (R8)
//
// The scene depth renderbuffer is shared to preserve depth testing behavior.
func (r *Renderer) ensureOITFramebuffers(w, h, samples int) error {
	if r.oitFB.width == w && r.oitFB.height == h && r.oitFB.samples == samples && r.oitFB.fbo != 0 {
		return nil
	}

	r.destroyOITFramebuffers()

	target := uint32(gl.TEXTURE_2D)
	if samples > 1 {
		target = gl.TEXTURE_2D_MULTISAMPLE
	}

	gl.GenTextures(1, &r.oitFB.accumTex)
	gl.BindTexture(target, r.oitFB.accumTex)
	if samples > 1 {
		gl.TexImage2DMultisample(target, int32(samples), gl.RGBA16F, int32(w), int32(h), true)
	} else {
		gl.TexImage2D(target, 0, gl.RGBA16F, int32(w), int32(h), 0, gl.RGBA, gl.HALF_FLOAT, nil)
		gl.TexParameteri(target, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
		gl.TexParameteri(target, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
		gl.TexParameteri(target, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
		gl.TexParameteri(target, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	}

	gl.GenTextures(1, &r.oitFB.revealageTex)
	gl.BindTexture(target, r.oitFB.revealageTex)
	if samples > 1 {
		gl.TexImage2DMultisample(target, int32(samples), gl.R8, int32(w), int32(h), true)
	} else {
		gl.TexImage2D(target, 0, gl.R8, int32(w), int32(h), 0, gl.RED, gl.UNSIGNED_BYTE, nil)
		gl.TexParameteri(target, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
		gl.TexParameteri(target, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
		gl.TexParameteri(target, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
		gl.TexParameteri(target, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	}
	gl.BindTexture(target, 0)

	gl.GenFramebuffers(1, &r.oitFB.fbo)
	gl.BindFramebuffer(gl.FRAMEBUFFER, r.oitFB.fbo)
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, target, r.oitFB.accumTex, 0)
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT1, target, r.oitFB.revealageTex, 0)
	if r.sceneDepthRBO != 0 {
		gl.FramebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER, r.sceneDepthRBO)
	}

	drawBuffers := [2]uint32{gl.COLOR_ATTACHMENT0, gl.COLOR_ATTACHMENT1}
	gl.DrawBuffers(2, &drawBuffers[0])

	if status := gl.CheckFramebufferStatus(gl.FRAMEBUFFER); status != gl.FRAMEBUFFER_COMPLETE {
		gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
		r.destroyOITFramebuffers()
		return fmt.Errorf("OIT framebuffer incomplete: status 0x%X", status)
	}

	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
	r.oitFB.width = w
	r.oitFB.height = h
	r.oitFB.samples = samples
	return nil
}

// destroyOITFramebuffers releases weighted blended OIT MRT resources.
func (r *Renderer) destroyOITFramebuffers() {
	if r.oitFB.fbo != 0 {
		gl.DeleteFramebuffers(1, &r.oitFB.fbo)
		r.oitFB.fbo = 0
	}
	if r.oitFB.accumTex != 0 {
		gl.DeleteTextures(1, &r.oitFB.accumTex)
		r.oitFB.accumTex = 0
	}
	if r.oitFB.revealageTex != 0 {
		gl.DeleteTextures(1, &r.oitFB.revealageTex)
		r.oitFB.revealageTex = 0
	}
	r.oitFB.width = 0
	r.oitFB.height = 0
	r.oitFB.samples = 0
}

// clearOITBuffers resets weighted blended OIT targets to the algorithm's
// required identity values: accum = 0, revealage = 1.
func (r *Renderer) clearOITBuffers() {
	if r.oitFB.fbo == 0 {
		return
	}

	var prevFBO int32
	gl.GetIntegerv(gl.FRAMEBUFFER_BINDING, &prevFBO)

	gl.BindFramebuffer(gl.FRAMEBUFFER, r.oitFB.fbo)
	drawBuffers := [2]uint32{gl.COLOR_ATTACHMENT0, gl.COLOR_ATTACHMENT1}
	gl.DrawBuffers(2, &drawBuffers[0])

	clearAccum := [4]float32{0, 0, 0, 0}
	gl.ClearBufferfv(gl.COLOR, 0, &clearAccum[0])

	clearReveal := [4]float32{1, 1, 1, 1}
	gl.ClearBufferfv(gl.COLOR, 1, &clearReveal[0])

	gl.BindFramebuffer(gl.FRAMEBUFFER, uint32(prevFBO))
}

// beginTranslucencyBlock enables frame-global translucent rendering state.
// In OIT mode this redirects translucent draws to the OIT MRT FBO and configures
// per-attachment blend equations for weighted blended accumulation.
func (r *Renderer) beginTranslucencyBlock() {
	alphaMode := GetAlphaMode()

	if alphaMode == AlphaModeOIT && r.oitFB.fbo != 0 {
		if err := r.ensureOITShaders(); err == nil {
			gl.BindFramebuffer(gl.FRAMEBUFFER, r.oitFB.fbo)
			drawBuffers := [2]uint32{gl.COLOR_ATTACHMENT0, gl.COLOR_ATTACHMENT1}
			gl.DrawBuffers(2, &drawBuffers[0])
			r.clearOITBuffers()

			gl.Enable(gl.BLEND)
			gl.BlendFunci(0, gl.ONE, gl.ONE)
			gl.BlendFunci(1, gl.ZERO, gl.ONE_MINUS_SRC_ALPHA)
			return
		}
	}

	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
}

// endTranslucencyBlock restores default framebuffer/blend state after translucent
// rendering and leaves room for the follow-up OIT resolve pass.
func (r *Renderer) endTranslucencyBlock() {
	alphaMode := GetAlphaMode()

	if alphaMode == AlphaModeOIT && r.oitFB.fbo != 0 {
		gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

		if r.sceneFBO != 0 {
			gl.BindFramebuffer(gl.FRAMEBUFFER, r.sceneFBO)
		} else {
			gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
		}

		// Resolve OIT accum/revealage into the scene framebuffer.
		resolveProgram := r.oitResolveProgram
		if r.oitFB.samples > 1 && r.oitResolveMSAAProgram != 0 {
			resolveProgram = r.oitResolveMSAAProgram
		}
		if resolveProgram != 0 {
			gl.Viewport(0, 0, int32(r.oitFB.width), int32(r.oitFB.height))
			gl.Disable(gl.DEPTH_TEST)
			gl.DepthMask(false)

			target := uint32(gl.TEXTURE_2D)
			if r.oitFB.samples > 1 {
				target = gl.TEXTURE_2D_MULTISAMPLE
			}
			gl.ActiveTexture(gl.TEXTURE0)
			gl.BindTexture(target, r.oitFB.accumTex)
			gl.ActiveTexture(gl.TEXTURE1)
			gl.BindTexture(target, r.oitFB.revealageTex)

			gl.UseProgram(resolveProgram)
			if resolveProgram == r.oitResolveMSAAProgram {
				gl.Uniform1i(r.oitResolveMSAASamplesLoc, int32(r.oitFB.samples))
			}

			gl.BindVertexArray(r.oitResolveVAO)
			gl.DrawArrays(gl.TRIANGLES, 0, 3)
			gl.BindVertexArray(0)

			gl.UseProgram(0)
			gl.ActiveTexture(gl.TEXTURE1)
			gl.BindTexture(target, 0)
			gl.ActiveTexture(gl.TEXTURE0)
			gl.BindTexture(target, 0)

			gl.Enable(gl.DEPTH_TEST)
		}
	}

	gl.DepthMask(true)
	gl.Disable(gl.BLEND)
}
