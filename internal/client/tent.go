package client

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/ironwail/ironwail-go/internal/common"
	inet "github.com/ironwail/ironwail-go/internal/net"
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
	start   [3]float32
	end     [3]float32
}

type TempEntityEvent struct {
	Type        byte
	Entity      int
	Origin      [3]float32
	Start       [3]float32
	End         [3]float32
	ColorStart  byte
	ColorLength byte
}

type BeamSegment struct {
	Type   byte
	Entity int
	Model  string
	Origin [3]float32
	Angles [3]float32
}

func (p *Parser) parseTempEntity(msg *common.SizeBuf) error {
	t, ok := msg.ReadByte()
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
		for i := 0; i < 3; i++ {
			coord, err := readCoord(msg, fmt.Sprintf("svc_temp_entity: missing origin %d", i))
			if err != nil {
				return err
			}
			event.Origin[i] = coord
		}

	case inet.TE_EXPLOSION2:
		for i := 0; i < 3; i++ {
			coord, err := readCoord(msg, fmt.Sprintf("svc_temp_entity: missing origin %d", i))
			if err != nil {
				return err
			}
			event.Origin[i] = coord
		}
		colorStart, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("svc_temp_entity: missing explosion2 color start")
		}
		event.ColorStart = colorStart
		colorLength, ok := msg.ReadByte()
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
		for i := 0; i < 3; i++ {
			coord, err := readCoord(msg, fmt.Sprintf("svc_temp_entity: missing beam start %d", i))
			if err != nil {
				return err
			}
			event.Start[i] = coord
		}
		for i := 0; i < 3; i++ {
			coord, err := readCoord(msg, fmt.Sprintf("svc_temp_entity: missing beam end %d", i))
			if err != nil {
				return err
			}
			event.End[i] = coord
		}

	default:
		return fmt.Errorf("svc_temp_entity: unsupported type %d", t)
	}

	p.Client.TempEntities = append(p.Client.TempEntities, event)
	appendTempEntitySound(p.Client, event)
	if isBeamType(event.Type) {
		p.Client.storeBeam(event)
	}
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
		return fmt.Sprintf("weapons/ric%d.wav", rand.Intn(3)+1)
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

func generateBeamSegments(typ byte, entity int, model string, start, end [3]float32) []BeamSegment {
	dist := [3]float32{
		end[0] - start[0],
		end[1] - start[1],
		end[2] - start[2],
	}
	length := sqrtFloat32(dist[0]*dist[0] + dist[1]*dist[1] + dist[2]*dist[2])
	if length == 0 {
		return []BeamSegment{{
			Type:   typ,
			Entity: entity,
			Model:  model,
			Origin: start,
		}}
	}
	dir := [3]float32{
		dist[0] / length,
		dist[1] / length,
		dist[2] / length,
	}

	yaw := float32(math.Atan2(float64(dir[1]), float64(dir[0])) * 180 / math.Pi)
	if yaw < 0 {
		yaw += 360
	}
	forward := sqrtFloat32(dir[0]*dir[0] + dir[1]*dir[1])
	pitch := float32(math.Atan2(float64(dir[2]), float64(forward)) * 180 / math.Pi)
	angles := [3]float32{pitch, yaw, 0}

	segments := make([]BeamSegment, 0, int(length/beamSegmentLength)+1)
	point := start
	for d := length; d > 0; d -= beamSegmentLength {
		segments = append(segments, BeamSegment{
			Type:   typ,
			Entity: entity,
			Model:  model,
			Origin: point,
			Angles: angles,
		})
		point[0] += dir[0] * beamSegmentLength
		point[1] += dir[1] * beamSegmentLength
		point[2] += dir[2] * beamSegmentLength
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
