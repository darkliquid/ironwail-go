package renderer

import (
	"unsafe"

	"github.com/darkliquid/ironwail-go/internal/renderer/particle"
)

const (
	MaxParticles         = particle.MaxParticles
	AbsoluteMinParticles = particle.AbsoluteMinParticles
)

type ParticleType = particle.ParticleType

const (
	ParticleStatic   = particle.ParticleStatic
	ParticleGrav     = particle.ParticleGrav
	ParticleSlowGrav = particle.ParticleSlowGrav
	ParticleFire     = particle.ParticleFire
	ParticleExplode  = particle.ParticleExplode
	ParticleExplode2 = particle.ParticleExplode2
	ParticleBlob     = particle.ParticleBlob
	ParticleBlob2    = particle.ParticleBlob2
)

type Particle = particle.Particle
type ParticleVertex = particle.ParticleVertex
type ParticleSystem = particle.ParticleSystem

var NewParticleSystem = particle.NewParticleSystem
var ParticleTexture = particle.ParticleTexture
var ShouldDrawParticles = particle.ShouldDrawParticles
var ParticleProjection = particle.ParticleProjection
var BuildParticleVertices = particle.BuildParticleVertices

func particleVertexPtr(vertices []ParticleVertex) unsafe.Pointer {
	return particle.ParticleVertexPtr(vertices)
}
