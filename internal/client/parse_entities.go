package client

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/common"
	inet "github.com/darkliquid/ironwail-go/internal/net"
)

func (p *Parser) parseSpawnBaseline(msg *common.SizeBuf, extended bool) error {
	baseline, entNum, err := p.readBaseline(msg, extended, true)
	if err != nil {
		return err
	}
	p.Client.EntityBaselines[entNum] = baseline
	return nil
}

func (p *Parser) parseSpawnStatic(msg *common.SizeBuf, extended bool) error {
	baseline, _, err := p.readBaseline(msg, extended, false)
	if err != nil {
		return err
	}
	p.Client.StaticEntities = append(p.Client.StaticEntities, baseline)
	return nil
}

func (p *Parser) readBaseline(msg *common.SizeBuf, extended bool, withEntNum bool) (inet.EntityState, int, error) {
	b := inet.EntityState{Alpha: inet.ENTALPHA_DEFAULT, Scale: inet.ENTSCALE_DEFAULT}
	prefix := "svc_spawnbaseline"
	if !withEntNum {
		prefix = "svc_spawnstatic"
	}

	var bits byte
	if extended {
		v, ok := msg.ReadByte()
		if !ok {
			return b, 0, fmt.Errorf("%s2: missing bits", prefix)
		}
		bits = v
	}

	entNum := 0
	if withEntNum {
		entNumRaw, ok := msg.ReadShort()
		if !ok {
			if extended {
				return b, 0, fmt.Errorf("%s2: missing entity", prefix)
			}
			return b, 0, fmt.Errorf("%s: missing entity", prefix)
		}
		entNum = int(entNumRaw)
	}

	if extended && bits&inet.BLARGEMODEL != 0 {
		v, ok := msg.ReadShort()
		if !ok {
			return b, 0, fmt.Errorf("%s2: missing modelindex", prefix)
		}
		b.ModelIndex = uint16(v)
	} else {
		v, ok := msg.ReadByte()
		if !ok {
			return b, 0, fmt.Errorf("%s: missing modelindex", prefix)
		}
		b.ModelIndex = uint16(v)
	}

	if extended && bits&inet.BLARGEFRAME != 0 {
		v, ok := msg.ReadShort()
		if !ok {
			return b, 0, fmt.Errorf("%s2: missing frame", prefix)
		}
		b.Frame = uint16(v)
	} else {
		v, ok := msg.ReadByte()
		if !ok {
			return b, 0, fmt.Errorf("%s: missing frame", prefix)
		}
		b.Frame = uint16(v)
	}

	colormap, ok := msg.ReadByte()
	if !ok {
		return b, 0, fmt.Errorf("%s: missing colormap", prefix)
	}
	b.Colormap = colormap

	skin, ok := msg.ReadByte()
	if !ok {
		return b, 0, fmt.Errorf("%s: missing skin", prefix)
	}
	b.Skin = skin

	// Origins and angles are interleaved: O1, A1, O2, A2, O3, A3
	for i := 0; i < 3; i++ {
		coord, err := p.readCoord(msg, fmt.Sprintf("%s: missing origin %d", prefix, i))
		if err != nil {
			return b, 0, err
		}
		b.Origin[i] = coord
		angle, err := p.readAngle(msg, fmt.Sprintf("%s: missing angle %d", prefix, i))
		if err != nil {
			return b, 0, err
		}
		b.Angles[i] = angle
	}
	b.MsgOrigins[0] = b.Origin
	b.MsgOrigins[1] = b.Origin
	b.MsgAngles[0] = b.Angles
	b.MsgAngles[1] = b.Angles

	if extended && bits&inet.BALPHA != 0 {
		alpha, ok := msg.ReadByte()
		if !ok {
			return b, 0, fmt.Errorf("%s2: missing alpha", prefix)
		}
		b.Alpha = alpha
	}
	if extended && bits&inet.BSCALE != 0 {
		scale, ok := msg.ReadByte()
		if !ok {
			return b, 0, fmt.Errorf("%s2: missing scale", prefix)
		}
		b.Scale = scale
	}

	return b, entNum, nil
}

func (p *Parser) parseSpawnStaticSound(msg *common.SizeBuf, extended bool) error {
	var snd StaticSound
	for i := 0; i < 3; i++ {
		coord, err := p.readCoord(msg, fmt.Sprintf("svc_spawnstaticsound: missing origin %d", i))
		if err != nil {
			return err
		}
		snd.Origin[i] = coord
	}
	if extended {
		v, ok := msg.ReadShort()
		if !ok {
			return fmt.Errorf("svc_spawnstaticsound2: missing sound index")
		}
		snd.SoundIndex = int(v)
	} else {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("svc_spawnstaticsound: missing sound index")
		}
		snd.SoundIndex = int(v)
	}
	volume, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_spawnstaticsound: missing volume")
	}
	attenuation, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_spawnstaticsound: missing attenuation")
	}
	snd.Volume = int(volume)
	snd.Attenuation = float32(attenuation) / 64
	p.Client.StaticSounds = append(p.Client.StaticSounds, snd)
	return nil
}

func (p *Parser) parseEntityUpdate(msg *common.SizeBuf, cmd byte) error {
	bits := uint32(cmd&0x7f) | inet.U_SIGNAL
	if bits&inet.U_MOREBITS != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("entity update: missing morebits")
		}
		bits |= uint32(v) << 8
	}
	// FitzQuake/RMQ protocols use bits 15+ as extension flags for additional
	// bytes. NetQuake reuses bit 15 as Nehahra's U_TRANS (transparency hack).
	if p.Client.Protocol != inet.PROTOCOL_NETQUAKE {
		if bits&inet.U_EXTEND1 != 0 {
			v, ok := msg.ReadByte()
			if !ok {
				return fmt.Errorf("entity update: missing extend1 bits")
			}
			bits |= uint32(v) << 16
		}
		if bits&inet.U_EXTEND2 != 0 {
			v, ok := msg.ReadByte()
			if !ok {
				return fmt.Errorf("entity update: missing extend2 bits")
			}
			bits |= uint32(v) << 24
		}
	}

	var entNum int
	if bits&inet.U_LONGENTITY != 0 {
		v, ok := msg.ReadShort()
		if !ok {
			return fmt.Errorf("entity update: missing long entity")
		}
		entNum = int(v)
	} else {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("entity update: missing entity")
		}
		entNum = int(v)
	}

	// Delta decode is baseline-relative: omitted fields inherit from baseline, not
	// from the entity's previous current state. Runtime interpolation/trail
	// bookkeeping is restored from the previous current state below.
	decode, ok := p.Client.EntityBaselines[entNum]
	if !ok {
		decode = inet.EntityState{Alpha: inet.ENTALPHA_DEFAULT, Scale: inet.ENTSCALE_DEFAULT}
	}
	state := decode
	isNew := true
	forceLink := true
	if current, ok := p.Client.Entities[entNum]; ok {
		isNew = false
		forceLink = current.MsgTime != p.Client.MTime[1] || current.ModelIndex == 0
		state.MsgOrigins = current.MsgOrigins
		state.MsgAngles = current.MsgAngles
		state.MsgTime = current.MsgTime
		state.ForceLink = current.ForceLink
		state.LerpFlags = current.LerpFlags
		state.TrailOrigin = current.TrailOrigin
		state.SpriteSyncBase = current.SpriteSyncBase
		state.SpriteSyncFrame = current.SpriteSyncFrame
		state.SpriteSyncModelIndex = current.SpriteSyncModelIndex
		state.Origin = current.Origin
		state.Angles = current.Angles
		state.LerpFinish = current.LerpFinish
	}
	rawOrigin := decode.Origin
	rawAngles := decode.Angles

	// Field read order must match C exactly (CL_ParseUpdate in cl_parse.c):
	// MODEL, FRAME, COLORMAP, SKIN, EFFECTS,
	// ORIGIN1, ANGLE1, ORIGIN2, ANGLE2, ORIGIN3, ANGLE3,
	// ALPHA, SCALE, FRAME2, MODEL2, LERPFINISH
	if bits&inet.U_MODEL != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("entity update: missing model")
		}
		decode.ModelIndex = uint16(v)
	}
	if bits&inet.U_FRAME != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("entity update: missing frame")
		}
		decode.Frame = uint16(v)
	}
	if bits&inet.U_COLORMAP != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("entity update: missing colormap")
		}
		decode.Colormap = v
	}
	if bits&inet.U_SKIN != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("entity update: missing skin")
		}
		decode.Skin = v
	}
	if bits&inet.U_EFFECTS != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("entity update: missing effects")
		}
		decode.Effects = int(v)
	}
	// Origins and angles are INTERLEAVED: O1, A1, O2, A2, O3, A3
	if bits&inet.U_ORIGIN1 != 0 {
		v, err := p.readCoord(msg, "entity update: missing origin1")
		if err != nil {
			return err
		}
		rawOrigin[0] = v
	}
	if bits&inet.U_ANGLE1 != 0 {
		v, err := p.readAngle(msg, "entity update: missing angle1")
		if err != nil {
			return err
		}
		rawAngles[0] = v
	}
	if bits&inet.U_ORIGIN2 != 0 {
		v, err := p.readCoord(msg, "entity update: missing origin2")
		if err != nil {
			return err
		}
		rawOrigin[1] = v
	}
	if bits&inet.U_ANGLE2 != 0 {
		v, err := p.readAngle(msg, "entity update: missing angle2")
		if err != nil {
			return err
		}
		rawAngles[1] = v
	}
	if bits&inet.U_ORIGIN3 != 0 {
		v, err := p.readCoord(msg, "entity update: missing origin3")
		if err != nil {
			return err
		}
		rawOrigin[2] = v
	}
	if bits&inet.U_ANGLE3 != 0 {
		v, err := p.readAngle(msg, "entity update: missing angle3")
		if err != nil {
			return err
		}
		rawAngles[2] = v
	}
	// FitzQuake/RMQ extensions come AFTER origins/angles.
	// For NetQuake protocol, handle Nehahra U_TRANS transparency hack instead.
	if p.Client.Protocol != inet.PROTOCOL_NETQUAKE {
		if bits&inet.U_ALPHA != 0 {
			v, ok := msg.ReadByte()
			if !ok {
				return fmt.Errorf("entity update: missing alpha")
			}
			decode.Alpha = v
		}
		if bits&inet.U_SCALE != 0 {
			v, ok := msg.ReadByte()
			if !ok {
				return fmt.Errorf("entity update: missing scale")
			}
			decode.Scale = v
		}
		if bits&inet.U_FRAME2 != 0 {
			v, ok := msg.ReadByte()
			if !ok {
				return fmt.Errorf("entity update: missing frame2")
			}
			decode.Frame = (decode.Frame & 0x00ff) | (uint16(v) << 8)
		}
		if bits&inet.U_MODEL2 != 0 {
			v, ok := msg.ReadByte()
			if !ok {
				return fmt.Errorf("entity update: missing model2")
			}
			decode.ModelIndex = (decode.ModelIndex & 0x00ff) | (uint16(v) << 8)
		}
		if bits&inet.U_LERPFINISH != 0 {
			v, ok := msg.ReadByte()
			if !ok {
				return fmt.Errorf("entity update: missing lerpfinish")
			}
			state.LerpFinish = p.Client.MTime[0] + float64(v)/255.0
			state.LerpFlags |= inet.LerpFinish
		} else {
			state.LerpFlags &^= inet.LerpFinish
		}
	} else {
		// PROTOCOL_NETQUAKE: bit 15 is Nehahra's U_TRANS, not U_EXTEND1.
		// Read transparency floats if set. Mirrors C cl_parse.c Nehahra hack.
		if bits&inet.U_EXTEND1 != 0 {
			if !p.warnedNehahra {
				slog.Warn("nonstandard update bit, assuming Nehahra protocol")
				p.warnedNehahra = true
			}
			a, ok := msg.ReadFloat()
			if !ok {
				return fmt.Errorf("entity update: missing nehahra trans type")
			}
			b, ok := msg.ReadFloat()
			if !ok {
				return fmt.Errorf("entity update: missing nehahra alpha")
			}
			if a == 2 {
				// Fullbright flag (not used yet).
				if _, ok := msg.ReadFloat(); !ok {
					return fmt.Errorf("entity update: missing nehahra fullbright")
				}
			}
			decode.Alpha = inet.ENTALPHA_ENCODE(b)
		}
	}
	state.ModelIndex = decode.ModelIndex
	state.Frame = decode.Frame
	state.Colormap = decode.Colormap
	state.Skin = decode.Skin
	state.Effects = decode.Effects
	state.Alpha = decode.Alpha
	state.Scale = decode.Scale
	if state.ModelIndex == 0 {
		// Server sent ModelIndex=0 → explicit retire (equivalent to C's
		// ent->model = NULL). Keep the slot in the map so later deltas still
		// have a valid base state, but stamp it as updated this frame so relink
		// doesn't preserve the stale live render model.
		state.MsgTime = p.Client.MTime[0]
		state.ForceLink = false
		state.LerpFlags |= inet.LerpResetMove | inet.LerpResetAnim
		state.LerpFlags &^= inet.LerpFinish
		p.Client.Entities[entNum] = state
		return nil
	}

	// Long-gap animation reset: if more than 0.2 seconds since the last
	// message for this entity, reset animation lerp to avoid interpolating
	// from a stale frame. Mirrors C: if (ent->msgtime + 0.2 < cl.mtime[0])
	if state.MsgTime+0.2 < p.Client.MTime[0] {
		state.LerpFlags |= inet.LerpResetAnim
	}

	// Update interpolation double-buffer: shift previous position/angles, then
	// store the new network values in [0]. Mirrors C's entity_t msg_origins handling.
	// Keep live Origin/Angles untouched on normal updates; CL_RelinkEntities owns
	// the render state except on force-link snaps.
	state.MsgOrigins[1] = state.MsgOrigins[0]
	state.MsgAngles[1] = state.MsgAngles[0]
	state.MsgOrigins[0] = rawOrigin
	state.MsgAngles[0] = rawAngles
	state.MsgTime = p.Client.MTime[0]
	if isNew {
		// Brand new entity: initialize [1] to match [0] so no spurious lerp on first frame.
		state.MsgOrigins[1] = state.MsgOrigins[0]
		state.MsgAngles[1] = state.MsgAngles[0]
		forceLink = true
	}

	// U_STEP indicates a monster step-move entity; position should not be lerped.
	if bits&inet.U_STEP != 0 {
		state.LerpFlags |= inet.LerpMoveStep
	} else {
		state.LerpFlags &^= inet.LerpMoveStep
	}
	if forceLink {
		state.MsgOrigins[1] = state.MsgOrigins[0]
		state.MsgAngles[1] = state.MsgAngles[0]
		state.Origin = state.MsgOrigins[0]
		state.Angles = state.MsgAngles[0]
		state.ForceLink = true
	} else {
		state.ForceLink = false
	}

	p.Client.Entities[entNum] = state
	return nil
}

func readChar(msg *common.SizeBuf, errMsg string) (int8, error) {
	v, ok := msg.ReadByte()
	if !ok {
		return 0, errors.New(errMsg)
	}
	return int8(v), nil
}

// readCoord reads a coordinate using 16-bit fixed-point (default FitzQuake encoding).
// TODO: Support protocol flags for float/int32/24-bit coord formats.
func readCoord(msg *common.SizeBuf, errMsg string) (float32, error) {
	v, ok := msg.ReadShort()
	if !ok {
		return 0, errors.New(errMsg)
	}
	return float32(v) / 8.0, nil
}

// readAngle reads an angle as a single byte (default FitzQuake encoding).
// TODO: Support protocol flags for float/short angle formats.
func readAngle(msg *common.SizeBuf, errMsg string) (float32, error) {
	v, ok := msg.ReadByte()
	if !ok {
		return 0, errors.New(errMsg)
	}
	return float32(v) * (360.0 / 256.0), nil
}

func (p *Parser) readPrecacheList(msg *common.SizeBuf) ([]string, error) {
	list := make([]string, 0, 64)
	for {
		v := msg.ReadString()
		if v == "" {
			return list, nil
		}
		list = append(list, trimNUL(v))
	}
}
