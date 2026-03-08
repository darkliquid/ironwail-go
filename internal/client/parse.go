package client

import (
	"errors"
	"fmt"

	"github.com/ironwail/ironwail-go/internal/common"
	inet "github.com/ironwail/ironwail-go/internal/net"
)

const (
	statHealth = iota
	statFrags
	statWeapon
	statAmmo
	statArmor
	statWeaponFrame
	statShells
	statNails
	statRockets
	statCells
	statActiveWeapon
)

type Parser struct {
	Client *Client
}

func NewParser(c *Client) *Parser {
	return &Parser{Client: c}
}

func (p *Parser) ParseServerMessage(data []byte) error {
	if p == nil || p.Client == nil {
		return fmt.Errorf("nil parser or client")
	}

	msg := common.NewSizeBuf(len(data))
	if !msg.Write(data) {
		return fmt.Errorf("failed to load message bytes")
	}
	msg.BeginReading()

	for {
		cmd, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("unexpected end of message")
		}
		if cmd == 0xFF {
			p.Client.FinishDemoFrame()
			return nil
		}
		if cmd&0x80 != 0 {
			if err := p.parseEntityUpdate(msg, cmd); err != nil {
				return err
			}
			continue
		}

		switch cmd {
		case inet.SVCNop:
		case inet.SVCDisconnect:
			p.Client.setState(StateDisconnected)
			return fmt.Errorf("server disconnected")
		case inet.SVCTime:
			v, ok := msg.ReadFloat()
			if !ok {
				return fmt.Errorf("svc_time: missing float")
			}
			p.Client.MTime[1] = p.Client.MTime[0]
			p.Client.MTime[0] = float64(v)
			p.Client.FixAngle = false
		case inet.SVCPrint:
			_ = msg.ReadString()
		case inet.SVCUpdateStat:
			if err := p.parseUpdateStat(msg); err != nil {
				return err
			}
		case inet.SVCStuffText:
			p.parseStuffText(msg.ReadString())
		case inet.SVCVersion:
			if err := p.parseVersion(msg); err != nil {
				return err
			}
		case inet.SVCServerInfo:
			if err := p.parseServerInfo(msg); err != nil {
				return err
			}
		case inet.SVCSetView:
			v, ok := msg.ReadShort()
			if !ok {
				return fmt.Errorf("svc_setview: missing entity")
			}
			p.Client.ViewEntity = int(v)
		case inet.SVCCDTrack:
			cd, ok := msg.ReadByte()
			if !ok {
				return fmt.Errorf("svc_cdtrack: missing track")
			}
			loop, ok := msg.ReadByte()
			if !ok {
				return fmt.Errorf("svc_cdtrack: missing loop track")
			}
			p.Client.CDTrack = int(cd)
			p.Client.LoopTrack = int(loop)
		case inet.SVCSignOnNum:
			if err := p.parseSignOnNum(msg); err != nil {
				return err
			}
		case inet.SVCClientData:
			if err := p.parseClientData(msg); err != nil {
				return err
			}
		case inet.SVCSound:
			if err := p.parseSound(msg, false); err != nil {
				return err
			}
		case inet.SVCLocalSound:
			if err := p.parseSound(msg, true); err != nil {
				return err
			}
		case inet.SVCUpdateFrags:
			if err := p.parseUpdateFrags(msg); err != nil {
				return err
			}
		case inet.SVCDamage:
			if err := p.parseDamage(msg); err != nil {
				return err
			}
		case inet.SVCSpawnBaseline:
			if err := p.parseSpawnBaseline(msg, false); err != nil {
				return err
			}
		case inet.SVCSpawnBaseline2:
			if err := p.parseSpawnBaseline(msg, true); err != nil {
				return err
			}
		case inet.SVCSpawnStatic:
			if err := p.parseSpawnStatic(msg, false); err != nil {
				return err
			}
		case inet.SVCSpawnStatic2:
			if err := p.parseSpawnStatic(msg, true); err != nil {
				return err
			}
		case inet.SVCSpawnStaticSound:
			if err := p.parseSpawnStaticSound(msg, false); err != nil {
				return err
			}
		case inet.SVCSpawnStaticSound2:
			if err := p.parseSpawnStaticSound(msg, true); err != nil {
				return err
			}
		case inet.SVCParticle:
			if err := p.parseParticle(msg); err != nil {
				return err
			}
		case inet.SVCTempEntity:
			if err := p.parseTempEntity(msg); err != nil {
				return err
			}
		case inet.SVCLightStyle:
			if err := p.parseLightStyle(msg); err != nil {
				return err
			}
		case inet.SVCSetAngle:
			if err := p.parseSetAngle(msg); err != nil {
				return err
			}
		case inet.SVCSetPause:
			if err := p.parseSetPause(msg); err != nil {
				return err
			}
		case inet.SVCCenterPrint:
			p.Client.CenterPrint = msg.ReadString()
		case inet.SVCUpdateName:
			if err := p.parseUpdateName(msg); err != nil {
				return err
			}
		case inet.SVCStopSound:
			if err := p.parseStopSound(msg); err != nil {
				return err
			}
		case inet.SVCUpdateColors:
			if err := p.parseUpdateColors(msg); err != nil {
				return err
			}
		case inet.SVCKillMonster:
			p.Client.KillCount++
		case inet.SVCFoundSecret:
			p.Client.SecretCount++
		case inet.SVCSkyBox:
			p.Client.SkyboxName = msg.ReadString()
		case inet.SVCFog:
			if err := p.parseFog(msg); err != nil {
				return err
			}
		case inet.SVCIntermission:
			p.Client.Intermission = 1
			p.Client.CompletedTime = p.Client.Time
		case inet.SVCFinale:
			_ = msg.ReadString()
			p.Client.Intermission = 2
			p.Client.CompletedTime = p.Client.Time
		case inet.SVCCutScene:
			_ = msg.ReadString()
			p.Client.Intermission = 3
			p.Client.CompletedTime = p.Client.Time
		default:
			return fmt.Errorf("unsupported server command: %d", cmd)
		}
	}
}

func (p *Parser) parseVersion(msg *common.SizeBuf) error {
	v, ok := msg.ReadLong()
	if !ok {
		return fmt.Errorf("svc_version: missing protocol")
	}
	if !supportedProtocol(v) {
		return fmt.Errorf("unsupported protocol %d", v)
	}
	p.Client.Protocol = v
	return nil
}

func (p *Parser) parseServerInfo(msg *common.SizeBuf) error {
	p.Client.ClearState()
	p.Client.setState(StateDisconnected)

	v, ok := msg.ReadLong()
	if !ok {
		return fmt.Errorf("svc_serverinfo: missing protocol")
	}
	if !supportedProtocol(v) {
		return fmt.Errorf("svc_serverinfo: unsupported protocol %d", v)
	}
	p.Client.Protocol = v

	if p.Client.Protocol == inet.PROTOCOL_RMQ {
		flags, ok := msg.ReadLong()
		if !ok {
			return fmt.Errorf("svc_serverinfo: missing rmq protocol flags")
		}
		p.Client.ProtocolFlags = uint32(flags)
	}

	maxClients, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_serverinfo: missing maxclients")
	}
	if maxClients < 1 {
		return fmt.Errorf("svc_serverinfo: invalid maxclients %d", maxClients)
	}
	p.Client.MaxClients = int(maxClients)

	gametype, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_serverinfo: missing gametype")
	}
	p.Client.GameType = int(gametype)

	p.Client.LevelName = trimNUL(msg.ReadString())

	models, err := p.readPrecacheList(msg)
	if err != nil {
		return fmt.Errorf("svc_serverinfo models: %w", err)
	}
	p.Client.ModelPrecache = models
	if len(models) > 0 {
		p.Client.MapName = parseMapNameFromWorldModel(models[0])
	}

	sounds, err := p.readPrecacheList(msg)
	if err != nil {
		return fmt.Errorf("svc_serverinfo sounds: %w", err)
	}
	p.Client.SoundPrecache = sounds

	return p.Client.HandleServerInfo()
}

func (p *Parser) parseSignOnNum(msg *common.SizeBuf) error {
	v, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_signonnum: missing signon")
	}
	signon := int(v)
	if p.Client.State == StateDisconnected {
		return fmt.Errorf("svc_signonnum: received while disconnected")
	}
	if signon <= p.Client.Signon {
		return fmt.Errorf("svc_signonnum out-of-order: got %d at %d", signon, p.Client.Signon)
	}
	p.Client.Signon = signon
	if signon == Signons {
		p.Client.setState(StateActive)
	}
	return nil
}

func (p *Parser) parseLightStyle(msg *common.SizeBuf) error {
	i, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_lightstyle: missing index")
	}
	return p.Client.SetLightStyle(int(i), msg.ReadString())
}

func (p *Parser) parseSetAngle(msg *common.SizeBuf) error {
	for i := 0; i < 3; i++ {
		b, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("svc_setangle: missing component %d", i)
		}
		p.Client.ViewAngles[i] = float32(b) * (360.0 / 256.0)
	}
	p.Client.FixAngle = true
	return nil
}

func (p *Parser) parseStuffText(s string) {
	p.Client.StuffCmdBuf += s
}

func (p *Parser) parseUpdateStat(msg *common.SizeBuf) error {
	idx, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_updatestat: missing index")
	}
	v, ok := msg.ReadLong()
	if !ok {
		return fmt.Errorf("svc_updatestat: missing value")
	}
	if int(idx) < len(p.Client.Stats) {
		p.Client.Stats[idx] = int(v)
	}
	return nil
}

func (p *Parser) parseUpdateFrags(msg *common.SizeBuf) error {
	idx, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_updatefrags: missing client index")
	}
	v, ok := msg.ReadShort()
	if !ok {
		return fmt.Errorf("svc_updatefrags: missing frags")
	}
	if p.Client.Frags == nil {
		p.Client.Frags = make(map[int]int)
	}
	p.Client.Frags[int(idx)] = int(v)
	return nil
}

func (p *Parser) parseSetPause(msg *common.SizeBuf) error {
	v, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_setpause: missing flag")
	}
	p.Client.Paused = v != 0
	return nil
}

func (p *Parser) parseUpdateName(msg *common.SizeBuf) error {
	idx, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_updatename: missing player index")
	}
	name := msg.ReadString()
	if p.Client.PlayerNames == nil {
		p.Client.PlayerNames = make(map[int]string)
	}
	p.Client.PlayerNames[int(idx)] = name
	return nil
}

func (p *Parser) parseStopSound(msg *common.SizeBuf) error {
	entity, ok := msg.ReadShort()
	if !ok {
		return fmt.Errorf("svc_stopsound: missing entity")
	}
	channel, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_stopsound: missing channel")
	}
	p.Client.StopSoundEvents = append(p.Client.StopSoundEvents, StopSoundEvent{
		Entity:  int(entity),
		Channel: int(channel),
	})
	return nil
}

func (p *Parser) parseUpdateColors(msg *common.SizeBuf) error {
	idx, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_updatecolors: missing player index")
	}
	colors, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_updatecolors: missing colors")
	}
	if p.Client.PlayerColors == nil {
		p.Client.PlayerColors = make(map[int]byte)
	}
	p.Client.PlayerColors[int(idx)] = colors
	return nil
}

func (p *Parser) parseFog(msg *common.SizeBuf) error {
	density, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_fog: missing density")
	}
	r, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_fog: missing red")
	}
	g, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_fog: missing green")
	}
	b, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_fog: missing blue")
	}
	t, ok := msg.ReadFloat()
	if !ok {
		return fmt.Errorf("svc_fog: missing time")
	}
	if t < 0 {
		t = 0
	}
	oldDensity, oldColor := p.Client.CurrentFog()
	p.Client.fogOldDensity = oldDensity
	p.Client.fogOldColor = oldColor
	p.Client.FogDensity = density
	p.Client.FogColor = [3]byte{r, g, b}
	p.Client.FogTime = float32(t)
	p.Client.fogFadeTime = float32(t)
	p.Client.fogFadeDone = p.Client.Time + float64(t)
	return nil
}

func (p *Parser) parseDamage(msg *common.SizeBuf) error {
	save, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_damage: missing armor")
	}
	take, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_damage: missing blood")
	}
	p.Client.DamageSaved = int(save)
	p.Client.DamageTaken = int(take)
	for i := 0; i < 3; i++ {
		coord, err := readCoord(msg, fmt.Sprintf("svc_damage: missing origin %d", i))
		if err != nil {
			return err
		}
		p.Client.DamageOrigin[i] = coord
	}
	return nil
}

func (p *Parser) parseSound(msg *common.SizeBuf, local bool) error {
	fieldMask, ok := msg.ReadByte()
	if !ok {
		if local {
			return fmt.Errorf("svc_localsound: missing field mask")
		}
		return fmt.Errorf("svc_sound: missing field mask")
	}

	event := SoundEvent{
		Volume:      inet.DEFAULT_SOUND_PACKET_VOLUME,
		Attenuation: inet.DEFAULT_SOUND_PACKET_ATTENUATION,
		Local:       local,
	}

	if fieldMask&inet.SND_VOLUME != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("svc_sound: missing volume")
		}
		event.Volume = int(v)
	}
	if fieldMask&inet.SND_ATTENUATION != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("svc_sound: missing attenuation")
		}
		event.Attenuation = float32(v) / 64
	}

	if local {
		if fieldMask != 0 {
			v, ok := msg.ReadShort()
			if !ok {
				return fmt.Errorf("svc_localsound: missing sound index")
			}
			event.SoundIndex = int(v)
		} else {
			v, ok := msg.ReadByte()
			if !ok {
				return fmt.Errorf("svc_localsound: missing sound index")
			}
			event.SoundIndex = int(v)
		}
		p.Client.SoundEvents = append(p.Client.SoundEvents, event)
		return nil
	}

	if fieldMask&inet.SND_LARGEENTITY != 0 {
		entNum, ok := msg.ReadShort()
		if !ok {
			return fmt.Errorf("svc_sound: missing large entity")
		}
		channel, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("svc_sound: missing large channel")
		}
		event.Entity = int(entNum)
		event.Channel = int(channel)
	} else {
		packed, ok := msg.ReadShort()
		if !ok {
			return fmt.Errorf("svc_sound: missing entity/channel")
		}
		event.Entity = int(uint16(packed) >> 3)
		event.Channel = int(uint16(packed) & 7)
	}

	if fieldMask&inet.SND_LARGESOUND != 0 {
		v, ok := msg.ReadShort()
		if !ok {
			return fmt.Errorf("svc_sound: missing large sound index")
		}
		event.SoundIndex = int(v)
	} else {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("svc_sound: missing sound index")
		}
		event.SoundIndex = int(v)
	}

	for i := 0; i < 3; i++ {
		coord, err := readCoord(msg, fmt.Sprintf("svc_sound: missing origin %d", i))
		if err != nil {
			return err
		}
		event.Origin[i] = coord
	}

	p.Client.SoundEvents = append(p.Client.SoundEvents, event)
	return nil
}

func (p *Parser) parseParticle(msg *common.SizeBuf) error {
	var event ParticleEvent
	for i := 0; i < 3; i++ {
		coord, err := readCoord(msg, fmt.Sprintf("svc_particle: missing origin %d", i))
		if err != nil {
			return err
		}
		event.Origin[i] = coord
	}
	for i := 0; i < 3; i++ {
		v, err := readChar(msg, fmt.Sprintf("svc_particle: missing dir %d", i))
		if err != nil {
			return err
		}
		event.Dir[i] = float32(v) / 16
	}
	count, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_particle: missing count")
	}
	color, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_particle: missing color")
	}
	event.Count = int(count)
	if count == 255 {
		event.Count = 1024
	}
	event.Color = int(color)
	p.Client.ParticleEvents = append(p.Client.ParticleEvents, event)
	return nil
}

func (p *Parser) parseClientData(msg *common.SizeBuf) error {
	bits16, ok := msg.ReadShort()
	if !ok {
		return fmt.Errorf("svc_clientdata: missing bits")
	}

	bits := uint32(uint16(bits16))
	if bits&inet.SU_EXTEND1 != 0 {
		ext, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("svc_clientdata: missing extend1 bits")
		}
		bits |= uint32(ext) << 16
	}
	if bits&inet.SU_EXTEND2 != 0 {
		ext, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("svc_clientdata: missing extend2 bits")
		}
		bits |= uint32(ext) << 24
	}

	viewHeight := float32(inet.DEFAULT_VIEWHEIGHT)
	if bits&inet.SU_VIEWHEIGHT != 0 {
		v, err := readChar(msg, "svc_clientdata: missing viewheight")
		if err != nil {
			return err
		}
		viewHeight = float32(v)
	}
	p.Client.ViewHeight = viewHeight
	if bits&inet.SU_IDEALPITCH != 0 {
		if _, err := readChar(msg, "svc_clientdata: missing idealpitch"); err != nil {
			return err
		}
	}

	punch := [3]float32{}
	for i := 0; i < 3; i++ {
		if bits&(inet.SU_PUNCH1<<uint(i)) != 0 {
			v, err := readChar(msg, fmt.Sprintf("svc_clientdata: missing punch %d", i))
			if err != nil {
				return err
			}
			punch[i] = float32(v)
		}
		if bits&(inet.SU_VELOCITY1<<uint(i)) != 0 {
			v, err := readChar(msg, fmt.Sprintf("svc_clientdata: missing velocity %d", i))
			if err != nil {
				return err
			}
			p.Client.MVelocity[1][i] = p.Client.MVelocity[0][i]
			p.Client.MVelocity[0][i] = float32(v) * 16
			p.Client.Velocity[i] = p.Client.MVelocity[0][i]
		}
	}
	if punch != p.Client.PunchAngles[0] {
		p.Client.PunchAngles[1] = p.Client.PunchAngles[0]
		p.Client.PunchAngles[0] = punch
		p.Client.PunchTime = p.Client.Time
		if p.Client.PunchTime == 0 {
			p.Client.PunchTime = p.Client.MTime[0]
		}
	}
	p.Client.PunchAngle = punch

	p.Client.OnGround = bits&inet.SU_ONGROUND != 0
	p.Client.InWater = bits&inet.SU_INWATER != 0

	items, ok := msg.ReadLong()
	if !ok {
		return fmt.Errorf("svc_clientdata: missing items")
	}
	p.Client.Items = uint32(items)

	if bits&inet.SU_WEAPONFRAME != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("svc_clientdata: missing weapon frame")
		}
		p.Client.Stats[statWeaponFrame] = int(v)
	}
	if bits&inet.SU_ARMOR != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("svc_clientdata: missing armor")
		}
		p.Client.Stats[statArmor] = int(v)
	}
	if bits&inet.SU_WEAPON != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("svc_clientdata: missing weapon")
		}
		p.Client.Stats[statWeapon] = int(v)
	}

	health, ok := msg.ReadShort()
	if !ok {
		return fmt.Errorf("svc_clientdata: missing health")
	}
	p.Client.Stats[statHealth] = int(health)

	ammo, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_clientdata: missing ammo")
	}
	p.Client.Stats[statAmmo] = int(ammo)

	shells, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_clientdata: missing shells")
	}
	p.Client.Stats[statShells] = int(shells)

	nails, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_clientdata: missing nails")
	}
	p.Client.Stats[statNails] = int(nails)

	rockets, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_clientdata: missing rockets")
	}
	p.Client.Stats[statRockets] = int(rockets)

	cells, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_clientdata: missing cells")
	}
	p.Client.Stats[statCells] = int(cells)

	activeWeapon, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_clientdata: missing active weapon")
	}
	p.Client.Stats[statActiveWeapon] = int(activeWeapon)

	return nil
}

func (p *Parser) parseSpawnBaseline(msg *common.SizeBuf, extended bool) error {
	baseline, entNum, err := p.readBaseline(msg, extended, true)
	if err != nil {
		return err
	}
	p.Client.EntityBaselines[entNum] = baseline
	p.Client.Entities[entNum] = baseline
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

	for i := 0; i < 3; i++ {
		coord, err := readCoord(msg, fmt.Sprintf("%s: missing origin %d", prefix, i))
		if err != nil {
			return b, 0, err
		}
		b.Origin[i] = coord
	}
	for i := 0; i < 3; i++ {
		angle, err := readAngle(msg, fmt.Sprintf("%s: missing angle %d", prefix, i))
		if err != nil {
			return b, 0, err
		}
		b.Angles[i] = angle
	}

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
		coord, err := readCoord(msg, fmt.Sprintf("svc_spawnstaticsound: missing origin %d", i))
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

	state, ok := p.Client.EntityBaselines[entNum]
	if !ok {
		state = inet.EntityState{Alpha: inet.ENTALPHA_DEFAULT, Scale: inet.ENTSCALE_DEFAULT}
	}
	if current, ok := p.Client.Entities[entNum]; ok {
		state = current
	}

	if bits&inet.U_MODEL != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("entity update: missing model")
		}
		state.ModelIndex = uint16(v)
	}
	if bits&inet.U_MODEL2 != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("entity update: missing model2")
		}
		state.ModelIndex = (state.ModelIndex & 0x00ff) | (uint16(v) << 8)
	}
	if bits&inet.U_FRAME != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("entity update: missing frame")
		}
		state.Frame = uint16(v)
	}
	if bits&inet.U_FRAME2 != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("entity update: missing frame2")
		}
		state.Frame = (state.Frame & 0x00ff) | (uint16(v) << 8)
	}
	if bits&inet.U_COLORMAP != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("entity update: missing colormap")
		}
		state.Colormap = v
	}
	if bits&inet.U_SKIN != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("entity update: missing skin")
		}
		state.Skin = v
	}
	if bits&inet.U_EFFECTS != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("entity update: missing effects")
		}
		state.Effects = int(v)
	}

	if bits&inet.U_ORIGIN1 != 0 {
		v, err := readCoord(msg, "entity update: missing origin1")
		if err != nil {
			return err
		}
		state.Origin[0] = v
	}
	if bits&inet.U_ORIGIN2 != 0 {
		v, err := readCoord(msg, "entity update: missing origin2")
		if err != nil {
			return err
		}
		state.Origin[1] = v
	}
	if bits&inet.U_ORIGIN3 != 0 {
		v, err := readCoord(msg, "entity update: missing origin3")
		if err != nil {
			return err
		}
		state.Origin[2] = v
	}
	if bits&inet.U_ANGLE1 != 0 {
		v, err := readAngle(msg, "entity update: missing angle1")
		if err != nil {
			return err
		}
		state.Angles[0] = v
	}
	if bits&inet.U_ANGLE2 != 0 {
		v, err := readAngle(msg, "entity update: missing angle2")
		if err != nil {
			return err
		}
		state.Angles[1] = v
	}
	if bits&inet.U_ANGLE3 != 0 {
		v, err := readAngle(msg, "entity update: missing angle3")
		if err != nil {
			return err
		}
		state.Angles[2] = v
	}
	if bits&inet.U_ALPHA != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("entity update: missing alpha")
		}
		state.Alpha = v
	}
	if bits&inet.U_SCALE != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("entity update: missing scale")
		}
		state.Scale = v
	}
	if bits&inet.U_LERPFINISH != 0 {
		if _, ok := msg.ReadByte(); !ok {
			return fmt.Errorf("entity update: missing lerpfinish")
		}
	}
	if state.ModelIndex == 0 {
		delete(p.Client.Entities, entNum)
		return nil
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

func readCoord(msg *common.SizeBuf, errMsg string) (float32, error) {
	v, ok := msg.ReadFloat()
	if !ok {
		return 0, errors.New(errMsg)
	}
	return v, nil
}

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
