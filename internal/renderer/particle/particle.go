package particle

import (
	"math"
	"math/rand"
	"sync"
	"unsafe"

	"github.com/darkliquid/ironwail-go/internal/compatrand"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

const (
	MaxParticles         = 16384
	AbsoluteMinParticles = 512
)

var (
	ramp1 = [...]byte{0x6f, 0x6d, 0x6b, 0x69, 0x67, 0x65, 0x63, 0x61}
	ramp2 = [...]byte{0x6f, 0x6e, 0x6d, 0x6c, 0x6b, 0x6a, 0x68, 0x66}
	ramp3 = [...]byte{0x6d, 0x6b, 6, 5, 4, 3}

	entityParticleNormals = [...]types.Vec3{
		{X: -0.525731, Y: 0.000000, Z: 0.850651},
		{X: -0.442863, Y: 0.238856, Z: 0.864188},
		{X: -0.295242, Y: 0.000000, Z: 0.955423},
		{X: -0.309017, Y: 0.500000, Z: 0.809017},
		{X: -0.162460, Y: 0.262866, Z: 0.951056},
		{X: 0.000000, Y: 0.000000, Z: 1.000000},
		{X: 0.000000, Y: 0.850651, Z: 0.525731},
		{X: -0.147621, Y: 0.716567, Z: 0.681718},
		{X: 0.147621, Y: 0.716567, Z: 0.681718},
		{X: 0.000000, Y: 0.525731, Z: 0.850651},
		{X: 0.309017, Y: 0.500000, Z: 0.809017},
		{X: 0.525731, Y: 0.000000, Z: 0.850651},
		{X: 0.295242, Y: 0.000000, Z: 0.955423},
		{X: 0.442863, Y: 0.238856, Z: 0.864188},
		{X: 0.162460, Y: 0.262866, Z: 0.951056},
		{X: -0.681718, Y: 0.147621, Z: 0.716567},
		{X: -0.809017, Y: 0.309017, Z: 0.500000},
		{X: -0.587785, Y: 0.425325, Z: 0.688191},
		{X: -0.850651, Y: 0.525731, Z: 0.000000},
		{X: -0.864188, Y: 0.442863, Z: 0.238856},
		{X: -0.716567, Y: 0.681718, Z: 0.147621},
		{X: -0.688191, Y: 0.587785, Z: 0.425325},
		{X: -0.500000, Y: 0.809017, Z: 0.309017},
		{X: -0.238856, Y: 0.864188, Z: 0.442863},
		{X: -0.425325, Y: 0.688191, Z: 0.587785},
		{X: -0.716567, Y: 0.681718, Z: -0.147621},
		{X: -0.500000, Y: 0.809017, Z: -0.309017},
		{X: -0.525731, Y: 0.850651, Z: 0.000000},
		{X: 0.000000, Y: 0.850651, Z: -0.525731},
		{X: -0.238856, Y: 0.864188, Z: -0.442863},
		{X: 0.000000, Y: 0.955423, Z: -0.295242},
		{X: -0.262866, Y: 0.951056, Z: -0.162460},
		{X: 0.000000, Y: 1.000000, Z: 0.000000},
		{X: 0.000000, Y: 0.955423, Z: 0.295242},
		{X: -0.262866, Y: 0.951056, Z: 0.162460},
		{X: 0.238856, Y: 0.864188, Z: 0.442863},
		{X: 0.262866, Y: 0.951056, Z: 0.162460},
		{X: 0.500000, Y: 0.809017, Z: 0.309017},
		{X: 0.238856, Y: 0.864188, Z: -0.442863},
		{X: 0.262866, Y: 0.951056, Z: -0.162460},
		{X: 0.500000, Y: 0.809017, Z: -0.309017},
		{X: 0.850651, Y: 0.525731, Z: 0.000000},
		{X: 0.716567, Y: 0.681718, Z: 0.147621},
		{X: 0.716567, Y: 0.681718, Z: -0.147621},
		{X: 0.525731, Y: 0.850651, Z: 0.000000},
		{X: 0.425325, Y: 0.688191, Z: 0.587785},
		{X: 0.864188, Y: 0.442863, Z: 0.238856},
		{X: 0.688191, Y: 0.587785, Z: 0.425325},
		{X: 0.809017, Y: 0.309017, Z: 0.500000},
		{X: 0.681718, Y: 0.147621, Z: 0.716567},
		{X: 0.587785, Y: 0.425325, Z: 0.688191},
		{X: 0.955423, Y: 0.295242, Z: 0.000000},
		{X: 1.000000, Y: 0.000000, Z: 0.000000},
		{X: 0.951056, Y: 0.162460, Z: 0.262866},
		{X: 0.850651, Y: -0.525731, Z: 0.000000},
		{X: 0.955423, Y: -0.295242, Z: 0.000000},
		{X: 0.864188, Y: -0.442863, Z: 0.238856},
		{X: 0.951056, Y: -0.162460, Z: 0.262866},
		{X: 0.809017, Y: -0.309017, Z: 0.500000},
		{X: 0.681718, Y: -0.147621, Z: 0.716567},
		{X: 0.850651, Y: 0.000000, Z: 0.525731},
		{X: 0.864188, Y: 0.442863, Z: -0.238856},
		{X: 0.809017, Y: 0.309017, Z: -0.500000},
		{X: 0.951056, Y: 0.162460, Z: -0.262866},
		{X: 0.525731, Y: 0.000000, Z: -0.850651},
		{X: 0.681718, Y: 0.147621, Z: -0.716567},
		{X: 0.681718, Y: -0.147621, Z: -0.716567},
		{X: 0.850651, Y: 0.000000, Z: -0.525731},
		{X: 0.809017, Y: -0.309017, Z: -0.500000},
		{X: 0.864188, Y: -0.442863, Z: -0.238856},
		{X: 0.951056, Y: -0.162460, Z: -0.262866},
		{X: 0.147621, Y: 0.716567, Z: -0.681718},
		{X: 0.309017, Y: 0.500000, Z: -0.809017},
		{X: 0.425325, Y: 0.688191, Z: -0.587785},
		{X: 0.442863, Y: 0.238856, Z: -0.864188},
		{X: 0.587785, Y: 0.425325, Z: -0.688191},
		{X: 0.688191, Y: 0.587785, Z: -0.425325},
		{X: -0.147621, Y: 0.716567, Z: -0.681718},
		{X: -0.309017, Y: 0.500000, Z: -0.809017},
		{X: 0.000000, Y: 0.525731, Z: -0.850651},
		{X: -0.525731, Y: 0.000000, Z: -0.850651},
		{X: -0.442863, Y: 0.238856, Z: -0.864188},
		{X: -0.295242, Y: 0.000000, Z: -0.955423},
		{X: -0.162460, Y: 0.262866, Z: -0.951056},
		{X: 0.000000, Y: 0.000000, Z: -1.000000},
		{X: 0.295242, Y: 0.000000, Z: -0.955423},
		{X: 0.162460, Y: 0.262866, Z: -0.951056},
		{X: -0.442863, Y: -0.238856, Z: -0.864188},
		{X: -0.309017, Y: -0.500000, Z: -0.809017},
		{X: -0.162460, Y: -0.262866, Z: -0.951056},
		{X: 0.000000, Y: -0.850651, Z: -0.525731},
		{X: -0.147621, Y: -0.716567, Z: -0.681718},
		{X: 0.147621, Y: -0.716567, Z: -0.681718},
		{X: 0.000000, Y: -0.525731, Z: -0.850651},
		{X: 0.309017, Y: -0.500000, Z: -0.809017},
		{X: 0.442863, Y: -0.238856, Z: -0.864188},
		{X: 0.162460, Y: -0.262866, Z: -0.951056},
		{X: 0.238856, Y: -0.864188, Z: -0.442863},
		{X: 0.500000, Y: -0.809017, Z: -0.309017},
		{X: 0.425325, Y: -0.688191, Z: -0.587785},
		{X: 0.716567, Y: -0.681718, Z: -0.147621},
		{X: 0.688191, Y: -0.587785, Z: -0.425325},
		{X: 0.587785, Y: -0.425325, Z: -0.688191},
		{X: 0.000000, Y: -0.955423, Z: -0.295242},
		{X: 0.000000, Y: -1.000000, Z: 0.000000},
		{X: 0.262866, Y: -0.951056, Z: -0.162460},
		{X: 0.000000, Y: -0.850651, Z: 0.525731},
		{X: 0.000000, Y: -0.955423, Z: 0.295242},
		{X: 0.238856, Y: -0.864188, Z: 0.442863},
		{X: 0.262866, Y: -0.951056, Z: 0.162460},
		{X: 0.500000, Y: -0.809017, Z: 0.309017},
		{X: 0.716567, Y: -0.681718, Z: 0.147621},
		{X: 0.525731, Y: -0.850651, Z: 0.000000},
		{X: -0.238856, Y: -0.864188, Z: -0.442863},
		{X: -0.500000, Y: -0.809017, Z: -0.309017},
		{X: -0.262866, Y: -0.951056, Z: -0.162460},
		{X: -0.850651, Y: -0.525731, Z: 0.000000},
		{X: -0.716567, Y: -0.681718, Z: -0.147621},
		{X: -0.716567, Y: -0.681718, Z: 0.147621},
		{X: -0.525731, Y: -0.850651, Z: 0.000000},
		{X: -0.500000, Y: -0.809017, Z: 0.309017},
		{X: -0.238856, Y: -0.864188, Z: 0.442863},
		{X: -0.262866, Y: -0.951056, Z: 0.162460},
		{X: -0.864188, Y: -0.442863, Z: 0.238856},
		{X: -0.809017, Y: -0.309017, Z: 0.500000},
		{X: -0.688191, Y: -0.587785, Z: 0.425325},
		{X: -0.681718, Y: -0.147621, Z: 0.716567},
		{X: -0.442863, Y: -0.238856, Z: 0.864188},
		{X: -0.587785, Y: -0.425325, Z: 0.688191},
		{X: -0.309017, Y: -0.500000, Z: 0.809017},
		{X: -0.147621, Y: -0.716567, Z: 0.681718},
		{X: -0.425325, Y: -0.688191, Z: 0.587785},
		{X: -0.162460, Y: -0.262866, Z: 0.951056},
		{X: 0.442863, Y: -0.238856, Z: 0.864188},
		{X: 0.162460, Y: -0.262866, Z: 0.951056},
		{X: 0.309017, Y: -0.500000, Z: 0.809017},
		{X: 0.147621, Y: -0.716567, Z: 0.681718},
		{X: 0.000000, Y: -0.525731, Z: 0.850651},
		{X: 0.425325, Y: -0.688191, Z: 0.587785},
		{X: 0.587785, Y: -0.425325, Z: 0.688191},
		{X: 0.688191, Y: -0.587785, Z: 0.425325},
		{X: -0.955423, Y: 0.295242, Z: 0.000000},
		{X: -0.951056, Y: 0.162460, Z: 0.262866},
		{X: -1.000000, Y: 0.000000, Z: 0.000000},
		{X: -0.850651, Y: 0.000000, Z: 0.525731},
		{X: -0.955423, Y: -0.295242, Z: 0.000000},
		{X: -0.951056, Y: -0.162460, Z: 0.262866},
		{X: -0.864188, Y: 0.442863, Z: -0.238856},
		{X: -0.951056, Y: 0.162460, Z: -0.262866},
		{X: -0.809017, Y: 0.309017, Z: -0.500000},
		{X: -0.864188, Y: -0.442863, Z: -0.238856},
		{X: -0.951056, Y: -0.162460, Z: -0.262866},
		{X: -0.809017, Y: -0.309017, Z: -0.500000},
		{X: -0.681718, Y: 0.147621, Z: -0.716567},
		{X: -0.681718, Y: -0.147621, Z: -0.716567},
		{X: -0.850651, Y: 0.000000, Z: -0.525731},
		{X: -0.688191, Y: 0.587785, Z: -0.425325},
		{X: -0.587785, Y: 0.425325, Z: -0.688191},
		{X: -0.425325, Y: 0.688191, Z: -0.587785},
		{X: -0.425325, Y: -0.688191, Z: -0.587785},
		{X: -0.587785, Y: -0.425325, Z: -0.688191},
		{X: -0.688191, Y: -0.587785, Z: -0.425325},
	}
	entityParticleAngularVelocities [len(entityParticleNormals)]types.Vec3
	entityParticleAngularVelOnce    sync.Once
)

// initEntityParticleAngularVelocities seeds deterministic spin vectors used to vary particle billboard rotation and keep effects visually rich.
func initEntityParticleAngularVelocities() [len(entityParticleNormals)]types.Vec3 {
	var velocities [len(entityParticleNormals)]types.Vec3
	for i := range velocities {
		velocities[i].X = float32(compatrand.Int()&255) * 0.01
		velocities[i].Y = float32(compatrand.Int()&255) * 0.01
		velocities[i].Z = float32(compatrand.Int()&255) * 0.01
	}
	return velocities
}

func randIntCompat(rng *rand.Rand) int {
	if rng != nil {
		return rng.Int()
	}
	return int(compatrand.Int())
}

type ParticleType byte

const (
	ParticleStatic ParticleType = iota
	ParticleGrav
	ParticleSlowGrav
	ParticleFire
	ParticleExplode
	ParticleExplode2
	ParticleBlob
	ParticleBlob2
)

type Particle struct {
	Org   types.Vec3
	Color byte
	Type  ParticleType

	Spawn float32
	Die   float32
	Vel   types.Vec3
	Ramp  float32
}

type ParticleVertex struct {
	Pos   types.Vec3
	Color [4]byte
}

func ParticleVertexPtr(vertices []ParticleVertex) unsafe.Pointer {
	if len(vertices) == 0 {
		return nil
	}
	return unsafe.Pointer(&vertices[0])
}

type ParticleSystem struct {
	particles   []Particle
	active      int
	tracerCount int
}

// NewParticleSystem allocates the particle pool and freelists used by Quake effects, avoiding per-frame allocations in hot rendering paths.
func NewParticleSystem(requested int) *ParticleSystem {
	switch {
	case requested <= 0:
		requested = MaxParticles
	case requested < AbsoluteMinParticles:
		requested = AbsoluteMinParticles
	}

	return &ParticleSystem{particles: make([]Particle, requested)}
}

// Capacity reports total particle slots so emitters can budget effects and avoid overcommitting transient visuals.
func (ps *ParticleSystem) Capacity() int {
	if ps == nil {
		return 0
	}
	return len(ps.particles)
}

// ActiveCount reports currently living particles, useful for diagnostics and adaptive quality controls.
func (ps *ParticleSystem) ActiveCount() int {
	if ps == nil {
		return 0
	}
	return ps.active
}

// ActiveParticles returns the active particle slice used by render passes to build camera-facing quads.
func (ps *ParticleSystem) ActiveParticles() []Particle {
	if ps == nil || ps.active == 0 {
		return nil
	}
	out := make([]Particle, ps.active)
	copy(out, ps.particles[:ps.active])
	return out
}

// AllocParticle grabs one free particle slot and initializes lifecycle bookkeeping for a new effect element.
func (ps *ParticleSystem) AllocParticle(now float32) *Particle {
	if ps == nil || ps.active >= len(ps.particles) {
		return nil
	}
	p := &ps.particles[ps.active]
	ps.active++
	p.Spawn = now - 0.001
	return p
}

// Clear resets particle state between level loads or hard resets so stale effects do not leak into new scenes.
func (ps *ParticleSystem) Clear() {
	if ps == nil {
		return
	}
	ps.active = 0
	ps.tracerCount = 0
}

// ParticleTexture returns the texture handle used by particle passes, typically a small alpha mask sampled by billboard shaders.
func ParticleTexture(mode int) (uvScale, textureScaleFactor float32) {
	switch mode {
	case 1:
		return 1, 1.27
	default:
		return 0.25, 1.0
	}
}

// ShouldDrawParticles performs its step in the particle simulation/storage layer feeding billboard rendering passes; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func ShouldDrawParticles(mode int, alpha, showTris bool, activeParticles int) bool {
	if mode == 0 || activeParticles == 0 {
		return false
	}
	if !showTris && alpha != (mode != 2) {
		return false
	}
	return true
}

// ParticleProjection performs its step in the particle simulation/storage layer feeding billboard rendering passes; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func ParticleProjection(textureScaleFactor float32, matProj [16]float32) (scaleX, scaleY float32) {
	s := textureScaleFactor * 0.375
	return s * matProj[4], s * -matProj[9]
}

// BuildParticleVertices performs its step in the particle simulation/storage layer feeding billboard rendering passes; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func BuildParticleVertices(active []Particle, palette [256][4]byte, showTris bool) []ParticleVertex {
	if len(active) == 0 {
		return nil
	}
	v := make([]ParticleVertex, len(active))
	for i := range active {
		v[i].Pos = active[i].Org
		if showTris {
			v[i].Color = [4]byte{255, 255, 255, 255}
			continue
		}
		v[i].Color = palette[active[i].Color]
	}
	return v
}

// RunParticles performs its step in the particle simulation/storage layer feeding billboard rendering passes; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func (ps *ParticleSystem) RunParticles(timeNow, oldTime, gravity float32) {
	if ps == nil || ps.active == 0 {
		return
	}

	frameTime := timeNow - oldTime
	time1 := frameTime * 5
	time2 := frameTime * 10
	time3 := frameTime * 15
	grav := frameTime * gravity * 0.05
	dvel := 4 * frameTime

	active := 0
	for cur := 0; cur < ps.active; cur++ {
		p := ps.particles[cur]
		if p.Die < timeNow || p.Spawn > timeNow {
			continue
		}

		p.Org.X += p.Vel.X * frameTime
		p.Org.Y += p.Vel.Y * frameTime
		p.Org.Z += p.Vel.Z * frameTime

		switch p.Type {
		case ParticleStatic:
		case ParticleFire:
			p.Ramp += time1
			if p.Ramp >= 6 {
				p.Die = -1
			} else {
				p.Color = ramp3[int(p.Ramp)]
			}
			p.Vel.Z += grav
		case ParticleExplode:
			p.Ramp += time2
			if p.Ramp >= 8 {
				p.Die = -1
			} else {
				p.Color = ramp1[int(p.Ramp)]
			}
			p.Vel.X += p.Vel.X * dvel
			p.Vel.Y += p.Vel.Y * dvel
			p.Vel.Z += p.Vel.Z * dvel
			p.Vel.Z -= grav
		case ParticleExplode2:
			p.Ramp += time3
			if p.Ramp >= 8 {
				p.Die = -1
			} else {
				p.Color = ramp2[int(p.Ramp)]
			}
			p.Vel.X -= p.Vel.X * frameTime
			p.Vel.Y -= p.Vel.Y * frameTime
			p.Vel.Z -= p.Vel.Z * frameTime
			p.Vel.Z -= grav
		case ParticleBlob:
			p.Vel.X += p.Vel.X * dvel
			p.Vel.Y += p.Vel.Y * dvel
			p.Vel.Z += p.Vel.Z * dvel
			p.Vel.Z -= grav
		case ParticleBlob2:
			p.Vel.X -= p.Vel.X * dvel
			p.Vel.Y -= p.Vel.Y * dvel
			p.Vel.Z -= grav
		case ParticleGrav, ParticleSlowGrav:
			p.Vel.Z -= grav
		}

		ps.particles[active] = p
		active++
	}

	ps.active = active
}

// RunParticleEffect performs its step in the particle simulation/storage layer feeding billboard rendering passes; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func (ps *ParticleSystem) RunParticleEffect(org, dir types.Vec3, color byte, count int, rng *rand.Rand, timeNow float32) {
	for i := 0; i < count; i++ {
		p := ps.AllocParticle(timeNow)
		if p == nil {
			return
		}

		if count == 1024 {
			p.Die = timeNow + 5
			p.Color = ramp1[0]
			p.Ramp = float32(randIntCompat(rng) & 3)
			if i&1 == 1 {
				p.Type = ParticleExplode
			} else {
				p.Type = ParticleExplode2
			}
			p.Org.X = org.X + float32(randIntCompat(rng)%32-16)
			p.Org.Y = org.Y + float32(randIntCompat(rng)%32-16)
			p.Org.Z = org.Z + float32(randIntCompat(rng)%32-16)
			p.Vel.X = float32(randIntCompat(rng)%512 - 256)
			p.Vel.Y = float32(randIntCompat(rng)%512 - 256)
			p.Vel.Z = float32(randIntCompat(rng)%512 - 256)
			continue
		}

		p.Die = timeNow + 0.1*float32(randIntCompat(rng)%5)
		p.Color = (color &^ 7) + byte(randIntCompat(rng)&7)
		p.Type = ParticleSlowGrav
		p.Org.X = org.X + float32((randIntCompat(rng)&15)-8)
		p.Org.Y = org.Y + float32((randIntCompat(rng)&15)-8)
		p.Org.Z = org.Z + float32((randIntCompat(rng)&15)-8)
		p.Vel = dir.Scale(15)
	}
}

// EntityParticles performs its step in the particle simulation/storage layer feeding billboard rendering passes; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func (ps *ParticleSystem) EntityParticles(org types.Vec3, timeNow float32) {
	if ps == nil {
		return
	}
	entityParticleAngularVelOnce.Do(func() {
		entityParticleAngularVelocities = initEntityParticleAngularVelocities()
	})

	const (
		entityParticleDist       = 64
		entityParticleBeamLength = 16
	)

	for i, normal := range entityParticleNormals {
		p := ps.AllocParticle(timeNow)
		if p == nil {
			return
		}

		velocity := entityParticleAngularVelocities[i]
		sp, cp := math.Sincos(float64(timeNow * velocity.X))
		sy, cy := math.Sincos(float64(timeNow * velocity.Y))

		p.Die = timeNow + 0.01
		p.Color = 0x6f
		p.Type = ParticleExplode
		p.Org.X = org.X + normal.X*entityParticleDist + float32(cp*cy)*entityParticleBeamLength
		p.Org.Y = org.Y + normal.Y*entityParticleDist + float32(cp*sy)*entityParticleBeamLength
		p.Org.Z = org.Z + normal.Z*entityParticleDist + float32(-sp)*entityParticleBeamLength
	}
}

// ParticleExplosion2 performs its step in the particle simulation/storage layer feeding billboard rendering passes; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func (ps *ParticleSystem) ParticleExplosion2(org types.Vec3, colorStart, colorLength byte, rng *rand.Rand, timeNow float32) {
	if ps == nil || colorLength == 0 {
		return
	}

	colorMod := 0
	for i := 0; i < 512; i++ {
		p := ps.AllocParticle(timeNow)
		if p == nil {
			return
		}

		p.Die = timeNow + 0.3
		p.Color = colorStart + byte(colorMod%int(colorLength))
		colorMod++
		p.Type = ParticleBlob
		p.Org.X = org.X + float32(randIntCompat(rng)%32-16)
		p.Org.Y = org.Y + float32(randIntCompat(rng)%32-16)
		p.Org.Z = org.Z + float32(randIntCompat(rng)%32-16)
		p.Vel.X = float32(randIntCompat(rng)%512 - 256)
		p.Vel.Y = float32(randIntCompat(rng)%512 - 256)
		p.Vel.Z = float32(randIntCompat(rng)%512 - 256)
	}
}

// BlobExplosion performs its step in the particle simulation/storage layer feeding billboard rendering passes; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func (ps *ParticleSystem) BlobExplosion(org types.Vec3, rng *rand.Rand, timeNow float32) {
	if ps == nil {
		return
	}

	for i := 0; i < 1024; i++ {
		p := ps.AllocParticle(timeNow)
		if p == nil {
			return
		}

		p.Die = timeNow + 1 + float32(randIntCompat(rng)&8)*0.05
		if i&1 == 1 {
			p.Type = ParticleBlob
			p.Color = byte(66 + randIntCompat(rng)%6)
		} else {
			p.Type = ParticleBlob2
			p.Color = byte(150 + randIntCompat(rng)%6)
		}
		p.Org.X = org.X + float32(randIntCompat(rng)%32-16)
		p.Org.Y = org.Y + float32(randIntCompat(rng)%32-16)
		p.Org.Z = org.Z + float32(randIntCompat(rng)%32-16)
		p.Vel.X = float32(randIntCompat(rng)%512 - 256)
		p.Vel.Y = float32(randIntCompat(rng)%512 - 256)
		p.Vel.Z = float32(randIntCompat(rng)%512 - 256)
	}
}

// LavaSplash performs its step in the particle simulation/storage layer feeding billboard rendering passes; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func (ps *ParticleSystem) LavaSplash(org types.Vec3, rng *rand.Rand, timeNow float32) {
	if ps == nil {
		return
	}

	for i := -16; i < 16; i++ {
		for j := -16; j < 16; j++ {
			p := ps.AllocParticle(timeNow)
			if p == nil {
				return
			}

			p.Die = timeNow + 2 + float32(randIntCompat(rng)&31)*0.02
			p.Color = byte(224 + (randIntCompat(rng) & 7))
			p.Type = ParticleSlowGrav

			dir := types.Vec3{
				X: float32(j*8 + (randIntCompat(rng) & 7)),
				Y: float32(i*8 + (randIntCompat(rng) & 7)),
				Z: 256,
			}
			p.Org.X = org.X + dir.X
			p.Org.Y = org.Y + dir.Y
			p.Org.Z = org.Z + float32(randIntCompat(rng)&63)

			dir = dir.Normalize()
			vel := float32(50 + (randIntCompat(rng) & 63))
			p.Vel = dir.Scale(vel)
		}
	}
}

// TeleportSplash performs its step in the particle simulation/storage layer feeding billboard rendering passes; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func (ps *ParticleSystem) TeleportSplash(org types.Vec3, rng *rand.Rand, timeNow float32) {
	if ps == nil {
		return
	}

	for i := -16; i < 16; i += 4 {
		for j := -16; j < 16; j += 4 {
			for k := -24; k < 32; k += 4 {
				p := ps.AllocParticle(timeNow)
				if p == nil {
					return
				}

				p.Die = timeNow + 0.2 + float32(randIntCompat(rng)&7)*0.02
				p.Color = byte(7 + (randIntCompat(rng) & 7))
				p.Type = ParticleSlowGrav

				dir := types.Vec3{X: float32(j * 8), Y: float32(i * 8), Z: float32(k * 8)}
				p.Org.X = org.X + float32(i+(randIntCompat(rng)&3))
				p.Org.Y = org.Y + float32(j+(randIntCompat(rng)&3))
				p.Org.Z = org.Z + float32(k+(randIntCompat(rng)&3))

				dir = dir.Normalize()
				vel := float32(50 + (randIntCompat(rng) & 63))
				p.Vel = dir.Scale(vel)
			}
		}
	}
}

// RocketTrail performs its step in the particle simulation/storage layer feeding billboard rendering passes; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func (ps *ParticleSystem) RocketTrail(start, end types.Vec3, typ int, rng *rand.Rand, timeNow float32) {
	vec := end.Sub(start)
	len := vec.Len()
	if len == 0 {
		return
	}
	vec = vec.Normalize()
	dec := float32(3)
	if typ >= 128 {
		dec = 1
		typ -= 128
	}

	for len > 0 {
		len -= dec

		p := ps.AllocParticle(timeNow)
		if p == nil {
			return
		}
		p.Vel = types.Vec3{}
		p.Die = timeNow + 2

		switch typ {
		case 0:
			p.Ramp = float32(randIntCompat(rng) & 3)
			p.Color = ramp3[int(p.Ramp)]
			p.Type = ParticleFire
			p.Org.X = start.X + float32(randIntCompat(rng)%6-3)
			p.Org.Y = start.Y + float32(randIntCompat(rng)%6-3)
			p.Org.Z = start.Z + float32(randIntCompat(rng)%6-3)
		case 1:
			p.Ramp = float32((randIntCompat(rng) & 3) + 2)
			p.Color = ramp3[int(p.Ramp)]
			p.Type = ParticleFire
			p.Org.X = start.X + float32(randIntCompat(rng)%6-3)
			p.Org.Y = start.Y + float32(randIntCompat(rng)%6-3)
			p.Org.Z = start.Z + float32(randIntCompat(rng)%6-3)
		case 2:
			p.Type = ParticleGrav
			p.Color = byte(67 + (randIntCompat(rng) & 3))
			p.Org.X = start.X + float32(randIntCompat(rng)%6-3)
			p.Org.Y = start.Y + float32(randIntCompat(rng)%6-3)
			p.Org.Z = start.Z + float32(randIntCompat(rng)%6-3)
		case 3, 5:
			p.Die = timeNow + 0.5
			p.Type = ParticleStatic
			if typ == 3 {
				p.Color = byte(52 + ((ps.tracerCount & 4) << 1))
			} else {
				p.Color = byte(230 + ((ps.tracerCount & 4) << 1))
			}
			ps.tracerCount++
			p.Org = start
			if ps.tracerCount&1 == 1 {
				p.Vel.X = 30 * vec.Y
				p.Vel.Y = -30 * vec.X
			} else {
				p.Vel.X = -30 * vec.Y
				p.Vel.Y = 30 * vec.X
			}
		case 4:
			p.Type = ParticleGrav
			p.Color = byte(67 + (randIntCompat(rng) & 3))
			p.Org.X = start.X + float32(randIntCompat(rng)%6-3)
			p.Org.Y = start.Y + float32(randIntCompat(rng)%6-3)
			p.Org.Z = start.Z + float32(randIntCompat(rng)%6-3)
			len -= 3
		case 6:
			p.Color = byte(9*16 + 8 + (randIntCompat(rng) & 3))
			p.Type = ParticleStatic
			p.Die = timeNow + 0.3
			p.Org.X = start.X + float32((randIntCompat(rng)&15)-8)
			p.Org.Y = start.Y + float32((randIntCompat(rng)&15)-8)
			p.Org.Z = start.Z + float32((randIntCompat(rng)&15)-8)
		}

		start = start.Add(vec)
	}
}
