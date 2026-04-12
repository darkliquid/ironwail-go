package client

import (
	"fmt"
	"math/bits"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/common"
	"github.com/darkliquid/ironwail-go/internal/console"
	inet "github.com/darkliquid/ironwail-go/internal/net"
)

func normalizeActiveWeapon(raw byte) int {
	if raw == 0 {
		return 0
	}
	if bits.OnesCount8(raw) == 1 {
		return int(raw)
	}
	if raw < 32 {
		return 1 << raw
	}
	return int(raw)
}

func (p *Parser) parseClientData(msg *common.SizeBuf, packetOffset int) error {
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
		idealPitch, err := readChar(msg, "svc_clientdata: missing idealpitch")
		if err != nil {
			return err
		}
		p.Client.IdealPitch = float32(idealPitch)
	} else {
		p.Client.IdealPitch = 0
	}
	if p.Client.Signon < Signons {
		p.Client.StopPitchDrift()
	}

	punch := [3]float32{}
	velocity := [3]float32{}
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
			velocity[i] = float32(v) * 16
		}
	}
	p.Client.MVelocity[1] = p.Client.MVelocity[0]
	p.Client.MVelocity[0] = velocity
	p.Client.Velocity = velocity
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

	p.Client.Stats[statWeaponFrame] = 0
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
	p.Client.Stats[statActiveWeapon] = normalizeActiveWeapon(activeWeapon)

	// FitzQuake extensions — high bytes for 16-bit stat values
	if bits&inet.SU_WEAPON2 != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("svc_clientdata: missing weapon2")
		}
		p.Client.Stats[statWeapon] |= int(v) << 8
	}
	if bits&inet.SU_ARMOR2 != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("svc_clientdata: missing armor2")
		}
		p.Client.Stats[statArmor] |= int(v) << 8
	}
	if bits&inet.SU_AMMO2 != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("svc_clientdata: missing ammo2")
		}
		p.Client.Stats[statAmmo] |= int(v) << 8
	}
	if bits&inet.SU_SHELLS2 != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("svc_clientdata: missing shells2")
		}
		p.Client.Stats[statShells] |= int(v) << 8
	}
	if bits&inet.SU_NAILS2 != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("svc_clientdata: missing nails2")
		}
		p.Client.Stats[statNails] |= int(v) << 8
	}
	if bits&inet.SU_ROCKETS2 != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("svc_clientdata: missing rockets2")
		}
		p.Client.Stats[statRockets] |= int(v) << 8
	}
	if bits&inet.SU_CELLS2 != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("svc_clientdata: missing cells2")
		}
		p.Client.Stats[statCells] |= int(v) << 8
	}
	if bits&inet.SU_WEAPONFRAME2 != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("svc_clientdata: missing weaponframe2")
		}
		p.Client.Stats[statWeaponFrame] |= int(v) << 8
	}
	if bits&inet.SU_WEAPONALPHA != 0 {
		v, ok := msg.ReadByte()
		if !ok {
			return fmt.Errorf("svc_clientdata: missing weaponalpha")
		}
		p.Client.ViewEntAlpha = v
	} else {
		p.Client.ViewEntAlpha = inet.ENTALPHA_DEFAULT
	}

	p.logSuspiciousClientData(msg, packetOffset, msg.ReadCount, bits, velocity, punch)

	return nil
}

func (p *Parser) recordPacketTrace(start, end int, name string) {
	p.packetTrace[p.traceCount%len(p.packetTrace)] = packetTraceEntry{
		name:  name,
		start: start,
		end:   end,
	}
	p.traceCount++
}

func (p *Parser) logSuspiciousClientData(msg *common.SizeBuf, start, end int, bits uint32, velocity, punch [3]float32) {
	if !isSuspiciousClientData(velocity, punch) {
		return
	}

	console.Printf(
		"client packet anomaly: current=%s[%d:%d] bits=0x%x onground=%t vel=%v punch=%v bytes=%s recent=%s\n",
		svcCommandName(inet.SVCClientData),
		start,
		end,
		bits,
		p.Client.OnGround,
		velocity,
		punch,
		formatPacketBytes(msg.Data[start:end]),
		p.packetTraceSummary(),
	)
}

func (p *Parser) packetTraceSummary() string {
	if p.traceCount == 0 {
		return "none"
	}
	count := p.traceCount
	if count > len(p.packetTrace) {
		count = len(p.packetTrace)
	}
	start := p.traceCount - count
	var b strings.Builder
	for i := start; i < p.traceCount; i++ {
		entry := p.packetTrace[i%len(p.packetTrace)]
		if b.Len() > 0 {
			b.WriteString(" | ")
		}
		fmt.Fprintf(&b, "%s[%d:%d]", entry.name, entry.start, entry.end)
	}
	return b.String()
}

func isSuspiciousClientData(velocity, punch [3]float32) bool {
	const suspiciousVelocity = 1000
	const suspiciousPunch = 90
	for _, v := range velocity {
		if abs32(v) > suspiciousVelocity {
			return true
		}
	}
	for _, v := range punch {
		if abs32(v) > suspiciousPunch {
			return true
		}
	}
	return false
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func formatPacketBytes(data []byte) string {
	if len(data) == 0 {
		return "-"
	}
	originalLen := len(data)
	const maxBytes = 24
	if len(data) > maxBytes {
		data = data[:maxBytes]
	}
	var b strings.Builder
	for i, v := range data {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%02x", v)
	}
	if originalLen > maxBytes {
		b.WriteString(" ...")
	}
	return b.String()
}

func svcCommandName(cmd byte) string {
	switch cmd {
	case inet.SVCNop:
		return "svc_nop"
	case inet.SVCDisconnect:
		return "svc_disconnect"
	case inet.SVCUpdateStat:
		return "svc_updatestat"
	case inet.SVCVersion:
		return "svc_version"
	case inet.SVCSetView:
		return "svc_setview"
	case inet.SVCSound:
		return "svc_sound"
	case inet.SVCTime:
		return "svc_time"
	case inet.SVCPrint:
		return "svc_print"
	case inet.SVCStuffText:
		return "svc_stufftext"
	case inet.SVCSetAngle:
		return "svc_setangle"
	case inet.SVCServerInfo:
		return "svc_serverinfo"
	case inet.SVCLightStyle:
		return "svc_lightstyle"
	case inet.SVCUpdateName:
		return "svc_updatename"
	case inet.SVCUpdateFrags:
		return "svc_updatefrags"
	case inet.SVCClientData:
		return "svc_clientdata"
	case inet.SVCStopSound:
		return "svc_stopsound"
	case inet.SVCUpdateColors:
		return "svc_updatecolors"
	case inet.SVCParticle:
		return "svc_particle"
	case inet.SVCDamage:
		return "svc_damage"
	case inet.SVCSpawnStatic:
		return "svc_spawnstatic"
	case inet.SVCSpawnBaseline:
		return "svc_spawnbaseline"
	case inet.SVCTempEntity:
		return "svc_temp_entity"
	case inet.SVCSetPause:
		return "svc_setpause"
	case inet.SVCSignOnNum:
		return "svc_signonnum"
	case inet.SVCCenterPrint:
		return "svc_centerprint"
	case inet.SVCKillMonster:
		return "svc_killmonster"
	case inet.SVCFoundSecret:
		return "svc_foundsecret"
	case inet.SVCSpawnStaticSound:
		return "svc_spawnstaticsound"
	case inet.SVCIntermission:
		return "svc_intermission"
	case inet.SVCFinale:
		return "svc_finale"
	case inet.SVCCDTrack:
		return "svc_cdtrack"
	case inet.SVCSellScreen:
		return "svc_sellscreen"
	case inet.SVCCutScene:
		return "svc_cutscene"
	case inet.SVCSkyBox:
		return "svc_skybox"
	case inet.SVCBF:
		return "svc_bf"
	case inet.SVCFog:
		return "svc_fog"
	case inet.SVCSpawnBaseline2:
		return "svc_spawnbaseline2"
	case inet.SVCSpawnStatic2:
		return "svc_spawnstatic2"
	case inet.SVCSpawnStaticSound2:
		return "svc_spawnstaticsound2"
	case inet.SVCAchievement:
		return "svc_achievement"
	case inet.SVCChat:
		return "svc_chat"
	case inet.SVCLevelCompleted:
		return "svc_levelcompleted"
	case inet.SVCBackToLobby:
		return "svc_backtolobby"
	case inet.SVCLocalSound:
		return "svc_localsound"
	default:
		return fmt.Sprintf("svc_%d", cmd)
	}
}
