package client

import (
	"fmt"

	"github.com/ironwail/ironwail-go/internal/common"
	inet "github.com/ironwail/ironwail-go/internal/net"
)

type TempEntityEvent struct {
	Type        byte
	Entity      int
	Origin      [3]float32
	Start       [3]float32
	End         [3]float32
	ColorStart  byte
	ColorLength byte
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
	return nil
}
