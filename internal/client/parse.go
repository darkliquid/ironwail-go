package client

import (
	"fmt"

	"github.com/darkliquid/ironwail-go/internal/common"
	"github.com/darkliquid/ironwail-go/internal/console"
	inet "github.com/darkliquid/ironwail-go/internal/net"
)

const (
	statHealth        = inet.StatHealth
	statFrags         = inet.StatFrags
	statWeapon        = inet.StatWeapon
	statAmmo          = inet.StatAmmo
	statArmor         = inet.StatArmor
	statWeaponFrame   = inet.StatWeaponFrame
	statShells        = inet.StatShells
	statNails         = inet.StatNails
	statRockets       = inet.StatRockets
	statCells         = inet.StatCells
	statActiveWeapon  = inet.StatActiveWeapon
	statTotalSecrets  = inet.StatTotalSecrets
	statTotalMonsters = inet.StatTotalMonsters
	statSecrets       = inet.StatSecrets
	statMonsters      = inet.StatMonsters
)

type Parser struct {
	Client        *Client
	warnedNehahra bool // Log Nehahra protocol warning only once per connection
	packetTrace   [8]packetTraceEntry
	traceCount    int
}

func NewParser(c *Client) *Parser {
	return &Parser{Client: c}
}

type packetTraceEntry struct {
	name  string
	start int
	end   int
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
	p.traceCount = 0

	for {
		cmdStart := msg.ReadCount
		cmd, ok := msg.GetByte()
		if !ok {
			p.Client.FinishDemoFrame()
			return nil
		}
		// Compatibility terminator used by some Go-side message builders.
		// C parses until byte exhaustion, but this codebase may append a trailing
		// 0xFF sentinel (sometimes followed by zero-padded capacity bytes from
		// SizeBuf.Data). Treat it as end-of-message only when there is no
		// remaining non-zero payload, so a legitimate fast-update command byte
		// 0xFF still parses correctly.
		if cmd == 0xFF && (msg.ReadCount == msg.CurSize || remainingBytesArePadding(msg)) {
			p.Client.FinishDemoFrame()
			return nil
		}
		if p.Client.Signon == Signons-1 && cmd != inet.SVCSignOnNum {
			p.Client.Signon = Signons
			p.Client.setState(StateActive)
		}
		if cmd&0x80 != 0 {
			if err := p.parseEntityUpdate(msg, cmd); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, "entity_update")
			continue
		}

		switch cmd {
		case inet.SVCBad:
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCNop:
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCDisconnect:
			p.Client.ClearState()
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
			if p.Client.Signon >= Signons && p.Client.State == StateActive {
				p.Client.acknowledgeCommand()
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCPrint:
			console.Printf("%s", msg.ReadString())
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCUpdateStat:
			if err := p.parseUpdateStat(msg); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCStuffText:
			p.parseStuffText(msg.ReadString())
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCVersion:
			if err := p.parseVersion(msg); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCServerInfo:
			if err := p.parseServerInfo(msg); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCSetView:
			v, ok := msg.ReadShort()
			if !ok {
				return fmt.Errorf("svc_setview: missing entity")
			}
			p.Client.ViewEntity = int(v)
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCCDTrack:
			cd, ok := msg.GetByte()
			if !ok {
				return fmt.Errorf("svc_cdtrack: missing track")
			}
			loop, ok := msg.GetByte()
			if !ok {
				return fmt.Errorf("svc_cdtrack: missing loop track")
			}
			p.Client.CDTrack = int(cd)
			p.Client.LoopTrack = int(loop)
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCSignOnNum:
			if err := p.parseSignOnNum(msg); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCClientData:
			if err := p.parseClientData(msg, cmdStart); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCSound:
			if err := p.parseSound(msg, false); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCLocalSound:
			if err := p.parseSound(msg, true); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCUpdateFrags:
			if err := p.parseUpdateFrags(msg); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCDamage:
			if err := p.parseDamage(msg); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCSpawnBaseline:
			if err := p.parseSpawnBaseline(msg, false); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCSpawnBaseline2:
			if err := p.parseSpawnBaseline(msg, true); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCSpawnStatic:
			if err := p.parseSpawnStatic(msg, false); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCSpawnStatic2:
			if err := p.parseSpawnStatic(msg, true); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCSpawnStaticSound:
			if err := p.parseSpawnStaticSound(msg, false); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCSpawnStaticSound2:
			if err := p.parseSpawnStaticSound(msg, true); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCParticle:
			if err := p.parseParticle(msg); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCTempEntity:
			if err := p.parseTempEntity(msg); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCLightStyle:
			if err := p.parseLightStyle(msg); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCSetAngle:
			if err := p.parseSetAngle(msg); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCSetPause:
			if err := p.parseSetPause(msg); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCCenterPrint:
			str := msg.ReadString()
			p.Client.CenterPrint = str
			p.Client.CenterPrintAt = p.Client.Time
			console.LogCenterPrint(p.Client.GameType, str)
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCUpdateName:
			if err := p.parseUpdateName(msg); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCStopSound:
			if err := p.parseStopSound(msg); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCUpdateColors:
			if err := p.parseUpdateColors(msg); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCKillMonster:
			p.Client.KillCount++
			p.Client.Stats[statMonsters]++
			p.Client.StatsF[statMonsters] = float32(p.Client.Stats[statMonsters])
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCFoundSecret:
			p.Client.SecretCount++
			p.Client.Stats[statSecrets]++
			p.Client.StatsF[statSecrets] = float32(p.Client.Stats[statSecrets])
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCSkyBox:
			p.Client.SkyboxName = msg.ReadString()
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCFog:
			if err := p.parseFog(msg); err != nil {
				return err
			}
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCIntermission:
			p.Client.Intermission = 1
			p.Client.CompletedTime = p.Client.Time
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCFinale:
			str := msg.ReadString()
			p.Client.CenterPrint = str
			p.Client.CenterPrintAt = p.Client.Time
			p.Client.Intermission = 2
			p.Client.CompletedTime = p.Client.Time
			console.LogCenterPrint(p.Client.GameType, str)
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCCutScene:
			str := msg.ReadString()
			p.Client.CenterPrint = str
			p.Client.CenterPrintAt = p.Client.Time
			p.Client.Intermission = 3
			p.Client.CompletedTime = p.Client.Time
			console.LogCenterPrint(p.Client.GameType, str)
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCSellScreen:
			// C: Cmd_ExecuteString("help", src_command) — no data to read
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCBF:
			p.Client.BonusFlash()
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCAchievement:
			_ = msg.ReadString() // achievement ID string, ignored
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		case inet.SVCLevelCompleted, inet.SVCBackToLobby:
			// Re-release protocol markers (no payload in classic streams).
			// C Ironwail advertises these opcodes and treats them as no-ops.
			p.recordPacketTrace(cmdStart, msg.ReadCount, svcCommandName(cmd))
		default:
			return fmt.Errorf("unsupported server command: %d", cmd)
		}
	}
}

func remainingBytesArePadding(msg *common.SizeBuf) bool {
	if msg == nil {
		return true
	}
	for i := msg.ReadCount; i < msg.CurSize; i++ {
		if msg.Data[i] != 0 {
			return false
		}
	}
	return true
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

	maxClients, ok := msg.GetByte()
	if !ok {
		return fmt.Errorf("svc_serverinfo: missing maxclients")
	}
	if maxClients < 1 {
		return fmt.Errorf("svc_serverinfo: invalid maxclients %d", maxClients)
	}
	p.Client.MaxClients = int(maxClients)

	gametype, ok := msg.GetByte()
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

	netDebugLogf("server_info", "protocol=%d flags=0x%08x maxclients=%d gametype=%d level=%q models=%d sounds=%d coord=%s angle=%s",
		p.Client.Protocol, p.Client.ProtocolFlags, p.Client.MaxClients,
		p.Client.GameType, p.Client.LevelName, len(models), len(sounds),
		coordEncodingName(p.Client.ProtocolFlags), angleEncodingName(p.Client.ProtocolFlags))

	return p.Client.HandleServerInfo()
}

// coordEncodingName returns a human label for the active coord
// encoding, matching the C enum ordering (PRFL_FLOATCOORD >
// PRFL_INT32COORD > PRFL_24BITCOORD > default 16-bit fixed).
func coordEncodingName(flags uint32) string {
	switch {
	case flags&inet.PRFL_FLOATCOORD != 0:
		return "float"
	case flags&inet.PRFL_INT32COORD != 0:
		return "int32"
	case flags&inet.PRFL_24BITCOORD != 0:
		return "24bit"
	}
	return "short16"
}

// angleEncodingName returns a human label for the active angle
// encoding (PRFL_FLOATANGLE > PRFL_SHORTANGLE > default byte).
func angleEncodingName(flags uint32) string {
	switch {
	case flags&inet.PRFL_FLOATANGLE != 0:
		return "float"
	case flags&inet.PRFL_SHORTANGLE != 0:
		return "short"
	}
	return "byte"
}

func (p *Parser) parseSignOnNum(msg *common.SizeBuf) error {
	v, ok := msg.GetByte()
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
	i, ok := msg.GetByte()
	if !ok {
		return fmt.Errorf("svc_lightstyle: missing index")
	}
	return p.Client.SetLightStyle(int(i), msg.ReadString())
}

func (p *Parser) parseSetAngle(msg *common.SizeBuf) error {
	for i := 0; i < 3; i++ {
		angle, err := p.readAngle(msg, fmt.Sprintf("svc_setangle: missing component %d", i))
		if err != nil {
			return err
		}
		p.Client.ViewAngles[i] = angle
	}
	p.Client.MViewAngles[0] = p.Client.ViewAngles
	p.Client.MViewAngles[1] = p.Client.ViewAngles
	p.Client.FixAngle = true
	return nil
}

func (p *Parser) readAngle(msg *common.SizeBuf, missingErr string) (float32, error) {
	if p != nil && p.Client != nil {
		switch {
		case p.Client.ProtocolFlags&inet.PRFL_FLOATANGLE != 0:
			v, ok := msg.ReadFloat()
			if !ok {
				return 0, fmt.Errorf("%s", missingErr)
			}
			return v, nil
		case p.Client.ProtocolFlags&inet.PRFL_SHORTANGLE != 0:
			v, ok := msg.ReadAngle16()
			if !ok {
				return 0, fmt.Errorf("%s", missingErr)
			}
			return v, nil
		}
	}
	v, ok := msg.ReadAngle()
	if !ok {
		return 0, fmt.Errorf("%s", missingErr)
	}
	return v, nil
}

func (p *Parser) readCoord(msg *common.SizeBuf, missingErr string) (float32, error) {
	if p != nil && p.Client != nil {
		switch {
		case p.Client.ProtocolFlags&inet.PRFL_FLOATCOORD != 0:
			v, ok := msg.ReadFloat()
			if !ok {
				return 0, fmt.Errorf("%s", missingErr)
			}
			return v, nil
		case p.Client.ProtocolFlags&inet.PRFL_INT32COORD != 0:
			v, ok := msg.ReadLong()
			if !ok {
				return 0, fmt.Errorf("%s", missingErr)
			}
			return float32(v) / 16.0, nil
		case p.Client.ProtocolFlags&inet.PRFL_24BITCOORD != 0:
			whole, ok := msg.ReadShort()
			if !ok {
				return 0, fmt.Errorf("%s", missingErr)
			}
			frac, ok := msg.GetByte()
			if !ok {
				return 0, fmt.Errorf("%s", missingErr)
			}
			return float32(whole) + float32(frac)/255.0, nil
		}
	}
	return readCoord(msg, missingErr)
}

func (p *Parser) parseStuffText(s string) {
	p.Client.StuffCmdBuf += s
}

func (p *Parser) parseUpdateStat(msg *common.SizeBuf) error {
	idx, ok := msg.GetByte()
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
	idx, ok := msg.GetByte()
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
	v, ok := msg.GetByte()
	if !ok {
		return fmt.Errorf("svc_setpause: missing flag")
	}
	p.Client.Paused = v != 0
	return nil
}

func (p *Parser) parseUpdateName(msg *common.SizeBuf) error {
	idx, ok := msg.GetByte()
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
	// C reads a single short: entity = i>>3, channel = i&7
	i, ok := msg.ReadShort()
	if !ok {
		return fmt.Errorf("svc_stopsound: missing data")
	}
	p.Client.StopSoundEvents = append(p.Client.StopSoundEvents, StopSoundEvent{
		Entity:  int(i >> 3),
		Channel: int(i & 7),
	})
	return nil
}

func (p *Parser) parseUpdateColors(msg *common.SizeBuf) error {
	idx, ok := msg.GetByte()
	if !ok {
		return fmt.Errorf("svc_updatecolors: missing player index")
	}
	colors, ok := msg.GetByte()
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
	density, ok := msg.GetByte()
	if !ok {
		return fmt.Errorf("svc_fog: missing density")
	}
	r, ok := msg.GetByte()
	if !ok {
		return fmt.Errorf("svc_fog: missing red")
	}
	g, ok := msg.GetByte()
	if !ok {
		return fmt.Errorf("svc_fog: missing green")
	}
	b, ok := msg.GetByte()
	if !ok {
		return fmt.Errorf("svc_fog: missing blue")
	}
	tRaw, ok := msg.ReadShort()
	if !ok {
		return fmt.Errorf("svc_fog: missing time")
	}
	t := float32(tRaw) / 100.0
	if t < 0 {
		t = 0
	}
	p.Client.SetFogState(density, [3]byte{r, g, b}, t)
	return nil
}

func (p *Parser) parseDamage(msg *common.SizeBuf) error {
	save, ok := msg.GetByte()
	if !ok {
		return fmt.Errorf("svc_damage: missing armor")
	}
	take, ok := msg.GetByte()
	if !ok {
		return fmt.Errorf("svc_damage: missing blood")
	}
	p.Client.DamageSaved = int(save)
	p.Client.DamageTaken = int(take)
	p.Client.FaceAnimUntil = p.Client.Time + 0.2
	for i := 0; i < 3; i++ {
		coord, err := p.readCoord(msg, fmt.Sprintf("svc_damage: missing origin %d", i))
		if err != nil {
			return err
		}
		p.Client.DamageOrigin[i] = coord
	}
	// Update the damage color shift (percent + color) from the new event.
	// Mirrors C view.c:V_ParseDamage() cshift update block.
	p.Client.ApplyDamage()
	return nil
}

func (p *Parser) parseSound(msg *common.SizeBuf, local bool) error {
	fieldMask, ok := msg.GetByte()
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
		v, ok := msg.GetByte()
		if !ok {
			return fmt.Errorf("svc_sound: missing volume")
		}
		event.Volume = int(v)
	}
	if fieldMask&inet.SND_ATTENUATION != 0 {
		v, ok := msg.GetByte()
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
			v, ok := msg.GetByte()
			if !ok {
				return fmt.Errorf("svc_localsound: missing sound index")
			}
			event.SoundIndex = int(v)
		}
		p.Client.SoundEvents = append(p.Client.SoundEvents, event)
		netDebugLogf("local_sound", "sound=%d volume=%d atten=%.3f",
			event.SoundIndex, event.Volume, event.Attenuation)
		return nil
	}

	if fieldMask&inet.SND_LARGEENTITY != 0 {
		entNum, ok := msg.ReadShort()
		if !ok {
			return fmt.Errorf("svc_sound: missing large entity")
		}
		channel, ok := msg.GetByte()
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
		v, ok := msg.GetByte()
		if !ok {
			return fmt.Errorf("svc_sound: missing sound index")
		}
		event.SoundIndex = int(v)
	}

	for i := 0; i < 3; i++ {
		coord, err := p.readCoord(msg, fmt.Sprintf("svc_sound: missing origin %d", i))
		if err != nil {
			return err
		}
		event.Origin[i] = coord
	}

	p.Client.SoundEvents = append(p.Client.SoundEvents, event)
	netDebugLogf("sound", "ent=%d ch=%d sound=%d volume=%d atten=%.3f origin=(%.3f %.3f %.3f) local=%t",
		event.Entity, event.Channel, event.SoundIndex, event.Volume, event.Attenuation,
		event.Origin[0], event.Origin[1], event.Origin[2], event.Local)
	return nil
}

func (p *Parser) parseParticle(msg *common.SizeBuf) error {
	var event ParticleEvent
	for i := 0; i < 3; i++ {
		coord, err := p.readCoord(msg, fmt.Sprintf("svc_particle: missing origin %d", i))
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
	count, ok := msg.GetByte()
	if !ok {
		return fmt.Errorf("svc_particle: missing count")
	}
	color, ok := msg.GetByte()
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
