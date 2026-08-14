package client

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/darkliquid/ironwail-go/internal/common"
	"github.com/darkliquid/ironwail-go/internal/compatrand"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

const (
	maxBeams          = 32
	beamSegmentLength = float32(30)
	beamLifetime      = 0.2
)

type beamState struct {
	entity  int
	typ     byte
	model   string
	endTime float64
	start   types.Vec3
	end     types.Vec3
}

type TempEntityEvent struct {
	Type        byte
	Entity      int
	Origin      types.Vec3
	Start       types.Vec3
	End         types.Vec3
	ColorStart  byte
	ColorLength byte
}

type BeamSegment struct {
	Type   byte
	Entity int
	Model  string
	Origin types.Vec3
	Angles types.Vec3
}

func (p *Parser) parseTempEntity(msg *common.SizeBuf) error {
	t, ok := msg.Byte()
	if !ok {
		return fmt.Errorf("svc_temp_entity: missing type")
	}

	event := TempEntityEvent{Type: t}

	switch t {
	case inet.TE_SPIKE,
		inet.TE_SUPERSPIKE,
		inet.TE_GUNSHOT,
		inet.TE_EXPLOSION,
		inet.TE_TAREXPLOSION,
		inet.TE_WIZSPIKE,
		inet.TE_KNIGHTSPIKE,
		inet.TE_LAVASPLASH,
		inet.TE_TELEPORT:
		x, err := p.readCoord(msg, "svc_temp_entity: missing origin 0")
		if err != nil {
			return err
		}
		y, err := p.readCoord(msg, "svc_temp_entity: missing origin 1")
		if err != nil {
			return err
		}
		z, err := p.readCoord(msg, "svc_temp_entity: missing origin 2")
		if err != nil {
			return err
		}
		event.Origin = types.Vec3{X: x, Y: y, Z: z}

	case inet.TE_EXPLOSION2:
		x, err := p.readCoord(msg, "svc_temp_entity: missing origin 0")
		if err != nil {
			return err
		}
		y, err := p.readCoord(msg, "svc_temp_entity: missing origin 1")
		if err != nil {
			return err
		}
		z, err := p.readCoord(msg, "svc_temp_entity: missing origin 2")
		if err != nil {
			return err
		}
		event.Origin = types.Vec3{X: x, Y: y, Z: z}
		colorStart, ok := msg.Byte()
		if !ok {
			return fmt.Errorf("svc_temp_entity: missing explosion2 color start")
		}
		event.ColorStart = colorStart
		colorLength, ok := msg.Byte()
		if !ok {
			return fmt.Errorf("svc_temp_entity: missing explosion2 color length")
		}
		event.ColorLength = colorLength

	case inet.TE_LIGHTNING1,
		inet.TE_LIGHTNING2,
		inet.TE_LIGHTNING3,
		inet.TE_BEAM:
		entNum, ok := msg.ReadShort()
		if !ok {
			return fmt.Errorf("svc_temp_entity: missing beam entity")
		}
		event.Entity = int(entNum)
		sx, err := p.readCoord(msg, "svc_temp_entity: missing beam start 0")
		if err != nil {
			return err
		}
		sy, err := p.readCoord(msg, "svc_temp_entity: missing beam start 1")
		if err != nil {
			return err
		}
		sz, err := p.readCoord(msg, "svc_temp_entity: missing beam start 2")
		if err != nil {
			return err
		}
		event.Start = types.Vec3{X: sx, Y: sy, Z: sz}

		ex, err := p.readCoord(msg, "svc_temp_entity: missing beam end 0")
		if err != nil {
			return err
		}
		ey, err := p.readCoord(msg, "svc_temp_entity: missing beam end 1")
		if err != nil {
			return err
		}
		ez, err := p.readCoord(msg, "svc_temp_entity: missing beam end 2")
		if err != nil {
			return err
		}
		event.End = types.Vec3{X: ex, Y: ey, Z: ez}

	default:
		return fmt.Errorf("svc_temp_entity: unsupported type %d", t)
	}

	p.Client.TempEntities = append(p.Client.TempEntities, event)
	appendTempEntitySound(p.Client, event)
	if isBeamType(event.Type) {
		p.Client.storeBeam(event)
	}
	netDebugLogf("tent", "type=%d entity=%d origin=(%.3f %.3f %.3f) start=(%.3f %.3f %.3f) end=(%.3f %.3f %.3f)",
		event.Type, event.Entity,
		event.Origin.X, event.Origin.Y, event.Origin.Z,
		event.Start.X, event.Start.Y, event.Start.Z,
		event.End.X, event.End.Y, event.End.Z)
	return nil
}

func appendTempEntitySound(c *Client, event TempEntityEvent) {
	if c == nil {
		return
	}
	soundName := tempEntitySoundName(event.Type)
	if soundName == "" {
		return
	}
	c.SoundEvents = append(c.SoundEvents, SoundEvent{
		Origin:      event.Origin,
		SoundName:   soundName,
		Volume:      255,
		Attenuation: 1,
	})
}

func tempEntitySoundName(typ byte) string {
	switch typ {
	case inet.TE_WIZSPIKE:
		return "wizard/hit.wav"
	case inet.TE_KNIGHTSPIKE:
		return "hknight/hit.wav"
	case inet.TE_TAREXPLOSION:
		return "weapons/r_exp3.wav"
	case inet.TE_SPIKE, inet.TE_SUPERSPIKE:
		if rand.Intn(5) != 0 {
			return "weapons/tink1.wav"
		}
		// C uses (rand() & 3) + 1, clamped to 3: values 0-2 map to ric1-ric3,
		// value 3 also maps to ric3, giving ric3 ~50% more probability.
		rnd := rand.Intn(4) + 1
		if rnd > 3 {
			rnd = 3
		}
		return fmt.Sprintf("weapons/ric%d.wav", rnd)
	default:
		return ""
	}
}

func (c *Client) storeBeam(event TempEntityEvent) {
	if c == nil || !isBeamType(event.Type) {
		return
	}
	model := beamModelName(event.Type)
	if model == "" {
		return
	}

	slot := -1
	for i := range c.beams {
		if c.beams[i].model != "" && c.beams[i].entity == event.Entity {
			slot = i
			break
		}
	}
	if slot < 0 {
		for i := range c.beams {
			if c.beams[i].model == "" || c.beams[i].endTime < c.Time {
				slot = i
				break
			}
		}
	}
	if slot < 0 {
		return
	}

	c.beams[slot] = beamState{
		entity:  event.Entity,
		typ:     event.Type,
		model:   model,
		endTime: c.Time + beamLifetime,
		start:   event.Start,
		end:     event.End,
	}
}

// UpdateTempEntities updates beam temp entities and generates beam segments for rendering.
func (c *Client) UpdateTempEntities() {
	if c == nil {
		return
	}
	c.BeamSegments = c.BeamSegments[:0]
	compatrand.ResetShared(int32(c.Time * 1000))
	for i := range c.beams {
		beam := c.beams[i]
		if beam.model == "" || beam.endTime < c.Time {
			continue
		}

		start := beam.start
		if beam.entity == c.ViewEntity {
			if state, ok := c.Entities[beam.entity]; ok {
				start = state.Origin
			}
		}
		c.BeamSegments = append(c.BeamSegments, generateBeamSegments(beam.typ, beam.entity, beam.model, start, beam.end)...)
	}
	if len(c.BeamSegments) == 0 {
		c.BeamSegments = nil
	}
}

func generateBeamSegments(typ byte, entity int, model string, start, end types.Vec3) []BeamSegment {
	nextRoll := func() float32 {
		return float32(compatrand.Int() % 360)
	}

	dist := end.Sub(start)
	length := dist.Len()
	if length == 0 {
		return []BeamSegment{{
			Type:   typ,
			Entity: entity,
			Model:  model,
			Origin: start,
			Angles: types.Vec3{X: 0, Y: 0, Z: nextRoll()},
		}}
	}
	dir := dist.Normalize()

	yaw := float32(math.Atan2(float64(dir.Y), float64(dir.X)) * 180 / math.Pi)
	if yaw < 0 {
		yaw += 360
	}
	forward := sqrtFloat32(dir.X*dir.X + dir.Y*dir.Y)
	pitch := float32(math.Atan2(float64(dir.Z), float64(forward)) * 180 / math.Pi)

	segments := make([]BeamSegment, 0, int(length/beamSegmentLength)+1)
	point := start
	for d := length; d > 0; d -= beamSegmentLength {
		segments = append(segments, BeamSegment{
			Type:   typ,
			Entity: entity,
			Model:  model,
			Origin: point,
			Angles: types.Vec3{X: pitch, Y: yaw, Z: nextRoll()},
		})
		point = point.Add(dir.Scale(beamSegmentLength))
	}
	return segments
}

func isBeamType(typ byte) bool {
	switch typ {
	case inet.TE_LIGHTNING1, inet.TE_LIGHTNING2, inet.TE_LIGHTNING3, inet.TE_BEAM:
		return true
	default:
		return false
	}
}

func beamModelName(typ byte) string {
	switch typ {
	case inet.TE_LIGHTNING1:
		return "progs/bolt.mdl"
	case inet.TE_LIGHTNING2:
		return "progs/bolt2.mdl"
	case inet.TE_LIGHTNING3:
		return "progs/bolt3.mdl"
	case inet.TE_BEAM:
		return "progs/beam.mdl"
	default:
		return ""
	}
}
