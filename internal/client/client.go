package client

import (
	"errors"
	"strings"

	"github.com/ironwail/ironwail-go/internal/common"
	inet "github.com/ironwail/ironwail-go/internal/net"
)

const (
	Signons           = 4
	defaultMaxPitch   = 90.0
	defaultMinPitch   = -90.0
	defaultWheelPitch = 5.0
)

type ClientState int

const (
	StateDisconnected ClientState = iota
	StateConnected
)

type KButton struct {
	Down  [2]int
	State int
}

type UserCmd struct {
	ViewAngles [3]float32
	Forward    float32
	Side       float32
	Up         float32
	Buttons    int
	Impulse    int
}

type LightStyle struct {
	Length  int
	Map     string
	Average byte
	Peak    byte
}

type Client struct {
	State  ClientState
	Signon int

	Protocol      int32
	ProtocolFlags uint32

	MTime   [2]float64
	Time    float64
	OldTime float64

	ViewAngles  [3]float32
	MViewAngles [2][3]float32
	MVelocity   [2][3]float32
	Velocity    [3]float32

	FixAngle bool

	MoveMessages int
	Cmd          UserCmd
	PendingCmd   UserCmd

	InImpulse int

	MaxClients int
	GameType   int

	LevelName string
	MapName   string

	ModelPrecache []string
	SoundPrecache []string

	ViewEntity int
	CDTrack    int
	LoopTrack  int

	Intermission  int
	CompletedTime float64

	Stats  [32]int
	StatsF [32]float32

	StuffCmdBuf string

	ForwardSpeed float32
	BackSpeed    float32
	SideSpeed    float32
	UpSpeed      float32

	YawSpeed      float32
	PitchSpeed    float32
	AngleSpeedKey float32
	MoveSpeedKey  float32

	AlwaysRun  bool
	FreeLook   bool
	LookSpring bool

	MaxPitch   float32
	MinPitch   float32
	WheelPitch float32

	InputForward   KButton
	InputBack      KButton
	InputLeft      KButton
	InputRight     KButton
	InputUp        KButton
	InputDown      KButton
	InputLookUp    KButton
	InputLookDown  KButton
	InputMoveLeft  KButton
	InputMoveRight KButton
	InputStrafe    KButton
	InputSpeed     KButton
	InputUse       KButton
	InputJump      KButton
	InputAttack    KButton
	InputKLook     KButton
	InputMLook     KButton

	LightStyles [64]LightStyle
}

func NewClient() *Client {
	return &Client{
		State:         StateDisconnected,
		ForwardSpeed:  200,
		BackSpeed:     200,
		SideSpeed:     350,
		UpSpeed:       200,
		YawSpeed:      140,
		PitchSpeed:    150,
		AngleSpeedKey: 1.5,
		MoveSpeedKey:  2.0,
		AlwaysRun:     true,
		FreeLook:      true,
		MaxPitch:      defaultMaxPitch,
		MinPitch:      defaultMinPitch,
		WheelPitch:    defaultWheelPitch,
	}
}

func (c *Client) ClearState() {
	c.Signon = 0
	c.Protocol = 0
	c.ProtocolFlags = 0
	c.MTime = [2]float64{}
	c.Time = 0
	c.OldTime = 0
	c.ViewEntity = 0
	c.CDTrack = 0
	c.LoopTrack = 0
	c.LevelName = ""
	c.MapName = ""
	c.ModelPrecache = nil
	c.SoundPrecache = nil
	c.StuffCmdBuf = ""
	c.Intermission = 0
	c.CompletedTime = 0
	c.FixAngle = false
	c.MoveMessages = 0
	c.InImpulse = 0
	c.PendingCmd = UserCmd{}
	c.Cmd = UserCmd{}
	c.Stats = [32]int{}
	c.StatsF = [32]float32{}
}

func (c *Client) SetLightStyle(i int, style string) error {
	if i < 0 || i >= len(c.LightStyles) {
		return errors.New("lightstyle index out of range")
	}
	ls := &c.LightStyles[i]
	ls.Map = style
	ls.Length = len(style)
	if ls.Length == 0 {
		ls.Average = 'm'
		ls.Peak = 'm'
		return nil
	}
	total := 0
	peak := byte('a')
	for j := 0; j < len(style); j++ {
		ch := style[j]
		total += int(ch - 'a')
		if ch > peak {
			peak = ch
		}
	}
	ls.Peak = peak
	ls.Average = byte(total/ls.Length) + 'a'
	return nil
}

func (c *Client) LerpPoint() float64 {
	f := c.MTime[0] - c.MTime[1]
	if f == 0 {
		c.Time = c.MTime[0]
		return 1
	}
	if f > 0.1 {
		c.MTime[1] = c.MTime[0] - 0.1
		f = 0.1
	}
	frac := (c.Time - c.MTime[1]) / f
	if frac < 0 {
		if frac < -0.01 {
			c.Time = c.MTime[1]
		}
		return 0
	}
	if frac > 1 {
		if frac > 1.01 {
			c.Time = c.MTime[0]
		}
		return 1
	}
	return frac
}

func (c *Client) SignonReply() {
	if c.Signon < 1 || c.Signon > Signons {
		return
	}
	if c.Signon == 4 {
		c.State = StateConnected
	}
}

func parseMapNameFromWorldModel(worldModel string) string {
	if worldModel == "" {
		return ""
	}
	return common.COM_StripExtension(common.COM_SkipPath(worldModel))
}

func supportedProtocol(version int32) bool {
	return version == inet.PROTOCOL_NETQUAKE || version == inet.PROTOCOL_FITZQUAKE || version == inet.PROTOCOL_RMQ
}

func trimNUL(s string) string {
	return strings.TrimRight(s, "\x00")
}
