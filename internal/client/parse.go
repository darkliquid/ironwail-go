package client

import (
	"fmt"

	"github.com/ironwail/ironwail-go/internal/common"
	inet "github.com/ironwail/ironwail-go/internal/net"
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

		switch cmd {
		case inet.SVCNop:
		case inet.SVCDisconnect:
			p.Client.State = StateDisconnected
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
		case inet.SVCLightStyle:
			if err := p.parseLightStyle(msg); err != nil {
				return err
			}
		case inet.SVCSetAngle:
			if err := p.parseSetAngle(msg); err != nil {
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

	p.Client.State = StateConnected
	return nil
}

func (p *Parser) parseSignOnNum(msg *common.SizeBuf) error {
	v, ok := msg.ReadByte()
	if !ok {
		return fmt.Errorf("svc_signonnum: missing signon")
	}
	signon := int(v)
	if signon <= p.Client.Signon {
		return fmt.Errorf("svc_signonnum out-of-order: got %d at %d", signon, p.Client.Signon)
	}
	p.Client.Signon = signon
	p.Client.SignonReply()
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
