package renderer

import (
	"math"
	"math/rand"
)

const (
	MaxParticles         = 16384
	AbsoluteMinParticles = 512
)

var (
	ramp1 = [...]byte{0x6f, 0x6d, 0x6b, 0x69, 0x67, 0x65, 0x63, 0x61}
	ramp2 = [...]byte{0x6f, 0x6e, 0x6d, 0x6c, 0x6b, 0x6a, 0x68, 0x66}
	ramp3 = [...]byte{0x6d, 0x6b, 6, 5, 4, 3}
)

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
	Org   [3]float32
	Color byte
	Type  ParticleType

	Spawn float32
	Die   float32
	Vel   [3]float32
	Ramp  float32
}

type ParticleVertex struct {
	Pos   [3]float32
	Color [4]byte
}

type ParticleSystem struct {
	particles []Particle
	active    int
}

func NewParticleSystem(requested int) *ParticleSystem {
	switch {
	case requested <= 0:
		requested = MaxParticles
	case requested < AbsoluteMinParticles:
		requested = AbsoluteMinParticles
	}

	return &ParticleSystem{particles: make([]Particle, requested)}
}

func (ps *ParticleSystem) Capacity() int {
	if ps == nil {
		return 0
	}
	return len(ps.particles)
}

func (ps *ParticleSystem) ActiveCount() int {
	if ps == nil {
		return 0
	}
	return ps.active
}

func (ps *ParticleSystem) ActiveParticles() []Particle {
	if ps == nil || ps.active == 0 {
		return nil
	}
	out := make([]Particle, ps.active)
	copy(out, ps.particles[:ps.active])
	return out
}

func (ps *ParticleSystem) AllocParticle(now float32) *Particle {
	if ps == nil || ps.active >= len(ps.particles) {
		return nil
	}
	p := &ps.particles[ps.active]
	ps.active++
	p.Spawn = now - 0.001
	return p
}

func (ps *ParticleSystem) Clear() {
	if ps == nil {
		return
	}
	ps.active = 0
}

func ParticleTexture(mode int) (uvScale, textureScaleFactor float32) {
	switch mode {
	case 1:
		return 1, 1.27
	default:
		return 0.25, 1.0
	}
}

func ShouldDrawParticles(mode int, alpha, showTris bool, activeParticles int) bool {
	if mode == 0 || activeParticles == 0 {
		return false
	}
	if !showTris && alpha != (mode != 2) {
		return false
	}
	return true
}

func ParticleProjection(textureScaleFactor float32, matProj [16]float32) (scaleX, scaleY float32) {
	s := textureScaleFactor * 0.375
	return s * matProj[4], s * -matProj[9]
}

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

		p.Org[0] += p.Vel[0] * frameTime
		p.Org[1] += p.Vel[1] * frameTime
		p.Org[2] += p.Vel[2] * frameTime

		switch p.Type {
		case ParticleStatic:
		case ParticleFire:
			p.Ramp += time1
			if p.Ramp >= 6 {
				p.Die = -1
			} else {
				p.Color = ramp3[int(p.Ramp)]
			}
			p.Vel[2] += grav
		case ParticleExplode:
			p.Ramp += time2
			if p.Ramp >= 8 {
				p.Die = -1
			} else {
				p.Color = ramp1[int(p.Ramp)]
			}
			for i := 0; i < 3; i++ {
				p.Vel[i] += p.Vel[i] * dvel
			}
			p.Vel[2] -= grav
		case ParticleExplode2:
			p.Ramp += time3
			if p.Ramp >= 8 {
				p.Die = -1
			} else {
				p.Color = ramp2[int(p.Ramp)]
			}
			for i := 0; i < 3; i++ {
				p.Vel[i] -= p.Vel[i] * frameTime
			}
			p.Vel[2] -= grav
		case ParticleBlob:
			for i := 0; i < 3; i++ {
				p.Vel[i] += p.Vel[i] * dvel
			}
			p.Vel[2] -= grav
		case ParticleBlob2:
			for i := 0; i < 2; i++ {
				p.Vel[i] -= p.Vel[i] * dvel
			}
			p.Vel[2] -= grav
		case ParticleGrav, ParticleSlowGrav:
			p.Vel[2] -= grav
		}

		ps.particles[active] = p
		active++
	}

	ps.active = active
}

func (ps *ParticleSystem) RunParticleEffect(org, dir [3]float32, color byte, count int, rng *rand.Rand, timeNow float32) {
	if rng == nil {
		rng = rand.New(rand.NewSource(1))
	}

	for i := 0; i < count; i++ {
		p := ps.AllocParticle(timeNow)
		if p == nil {
			return
		}

		if count == 1024 {
			p.Die = timeNow + 5
			p.Color = ramp1[0]
			p.Ramp = float32(rng.Intn(4))
			if i&1 == 1 {
				p.Type = ParticleExplode
			} else {
				p.Type = ParticleExplode2
			}
			for j := 0; j < 3; j++ {
				p.Org[j] = org[j] + float32(rng.Intn(32)-16)
				p.Vel[j] = float32(rng.Intn(512) - 256)
			}
			continue
		}

		p.Die = timeNow + 0.1*float32(rng.Intn(5))
		p.Color = (color &^ 7) + byte(rng.Intn(8))
		p.Type = ParticleSlowGrav
		for j := 0; j < 3; j++ {
			p.Org[j] = org[j] + float32((rng.Int()&15)-8)
			p.Vel[j] = dir[j] * 15
		}
	}
}

func (ps *ParticleSystem) ParticleExplosion2(org [3]float32, colorStart, colorLength byte, rng *rand.Rand, timeNow float32) {
	if ps == nil || colorLength == 0 {
		return
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(1))
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
		for j := 0; j < 3; j++ {
			p.Org[j] = org[j] + float32(rng.Intn(32)-16)
			p.Vel[j] = float32(rng.Intn(512) - 256)
		}
	}
}

func (ps *ParticleSystem) BlobExplosion(org [3]float32, rng *rand.Rand, timeNow float32) {
	if ps == nil {
		return
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(1))
	}

	for i := 0; i < 1024; i++ {
		p := ps.AllocParticle(timeNow)
		if p == nil {
			return
		}

		p.Die = timeNow + 1 + float32(rng.Int()&8)*0.05
		if i&1 == 1 {
			p.Type = ParticleBlob
			p.Color = byte(66 + rng.Intn(6))
		} else {
			p.Type = ParticleBlob2
			p.Color = byte(150 + rng.Intn(6))
		}
		for j := 0; j < 3; j++ {
			p.Org[j] = org[j] + float32(rng.Intn(32)-16)
			p.Vel[j] = float32(rng.Intn(512) - 256)
		}
	}
}

func (ps *ParticleSystem) LavaSplash(org [3]float32, rng *rand.Rand, timeNow float32) {
	if ps == nil {
		return
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(1))
	}

	for i := -16; i < 16; i++ {
		for j := -16; j < 16; j++ {
			p := ps.AllocParticle(timeNow)
			if p == nil {
				return
			}

			p.Die = timeNow + 2 + float32(rng.Int()&31)*0.02
			p.Color = byte(224 + (rng.Int() & 7))
			p.Type = ParticleSlowGrav

			dir := [3]float32{
				float32(j*8 + (rng.Int() & 7)),
				float32(i*8 + (rng.Int() & 7)),
				256,
			}
			p.Org[0] = org[0] + dir[0]
			p.Org[1] = org[1] + dir[1]
			p.Org[2] = org[2] + float32(rng.Int()&63)

			normalize3(&dir)
			vel := float32(50 + (rng.Int() & 63))
			for k := 0; k < 3; k++ {
				p.Vel[k] = dir[k] * vel
			}
		}
	}
}

func (ps *ParticleSystem) TeleportSplash(org [3]float32, rng *rand.Rand, timeNow float32) {
	if ps == nil {
		return
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(1))
	}

	for i := -16; i < 16; i += 4 {
		for j := -16; j < 16; j += 4 {
			for k := -24; k < 32; k += 4 {
				p := ps.AllocParticle(timeNow)
				if p == nil {
					return
				}

				p.Die = timeNow + 0.2 + float32(rng.Int()&7)*0.02
				p.Color = byte(7 + (rng.Int() & 7))
				p.Type = ParticleSlowGrav

				dir := [3]float32{float32(j * 8), float32(i * 8), float32(k * 8)}
				p.Org[0] = org[0] + float32(i+(rng.Int()&3))
				p.Org[1] = org[1] + float32(j+(rng.Int()&3))
				p.Org[2] = org[2] + float32(k+(rng.Int()&3))

				normalize3(&dir)
				vel := float32(50 + (rng.Int() & 63))
				for n := 0; n < 3; n++ {
					p.Vel[n] = dir[n] * vel
				}
			}
		}
	}
}

func (ps *ParticleSystem) RocketTrail(start, end [3]float32, typ int, rng *rand.Rand, timeNow float32) {
	if rng == nil {
		rng = rand.New(rand.NewSource(1))
	}

	vec := [3]float32{end[0] - start[0], end[1] - start[1], end[2] - start[2]}
	len := normalize3(&vec)
	dec := float32(3)
	if typ >= 128 {
		dec = 1
		typ -= 128
	}

	tracerCount := 0
	for len > 0 {
		len -= dec

		p := ps.AllocParticle(timeNow)
		if p == nil {
			return
		}
		p.Vel = [3]float32{}
		p.Die = timeNow + 2

		switch typ {
		case 0:
			p.Ramp = float32(rng.Intn(4))
			p.Color = ramp3[int(p.Ramp)]
			p.Type = ParticleFire
			for j := 0; j < 3; j++ {
				p.Org[j] = start[j] + float32(rng.Intn(6)-3)
			}
		case 1:
			p.Ramp = float32(rng.Intn(4) + 2)
			p.Color = ramp3[int(p.Ramp)]
			p.Type = ParticleFire
			for j := 0; j < 3; j++ {
				p.Org[j] = start[j] + float32(rng.Intn(6)-3)
			}
		case 2:
			p.Type = ParticleGrav
			p.Color = byte(67 + rng.Intn(4))
			for j := 0; j < 3; j++ {
				p.Org[j] = start[j] + float32(rng.Intn(6)-3)
			}
		case 3, 5:
			p.Die = timeNow + 0.5
			p.Type = ParticleStatic
			if typ == 3 {
				p.Color = byte(52 + ((tracerCount & 4) << 1))
			} else {
				p.Color = byte(230 + ((tracerCount & 4) << 1))
			}
			tracerCount++
			p.Org = start
			if tracerCount&1 == 1 {
				p.Vel[0] = 30 * vec[1]
				p.Vel[1] = -30 * vec[0]
			} else {
				p.Vel[0] = -30 * vec[1]
				p.Vel[1] = 30 * vec[0]
			}
		case 4:
			p.Type = ParticleGrav
			p.Color = byte(67 + rng.Intn(4))
			for j := 0; j < 3; j++ {
				p.Org[j] = start[j] + float32(rng.Intn(6)-3)
			}
			len -= 3
		case 6:
			p.Color = byte(9*16 + 8 + rng.Intn(4))
			p.Type = ParticleStatic
			p.Die = timeNow + 0.3
			for j := 0; j < 3; j++ {
				p.Org[j] = start[j] + float32((rng.Int()&15)-8)
			}
		}

		start[0] += vec[0]
		start[1] += vec[1]
		start[2] += vec[2]
	}
}

func normalize3(v *[3]float32) float32 {
	l := float32(math.Sqrt(float64(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])))
	if l == 0 {
		return 0
	}
	v[0] /= l
	v[1] /= l
	v[2] /= l
	return l
}
