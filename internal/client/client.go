package client

import (
	"errors"
	"fmt"
	"log"
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
	StateActive
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

type StaticSound struct {
	Origin      [3]float32
	SoundIndex  int
	Volume      int
	Attenuation float32
}

type SoundEvent struct {
	Entity      int
	Channel     int
	Origin      [3]float32
	SoundIndex  int
	Volume      int
	Attenuation float32
	Local       bool
}

type ParticleEvent struct {
	Origin [3]float32
	Dir    [3]float32
	Count  int
	Color  int
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
	Paused        bool
	CenterPrint   string

	Stats  [32]int
	StatsF [32]float32
	Items  uint32
	Frags  map[int]int

	OnGround bool
	InWater  bool

	EntityBaselines map[int]inet.EntityState
	Entities        map[int]inet.EntityState
	StaticEntities  []inet.EntityState
	StaticSounds    []StaticSound
	SoundEvents     []SoundEvent
	ParticleEvents  []ParticleEvent
	TempEntities    []TempEntityEvent
	DamageTaken     int
	DamageSaved     int
	DamageOrigin    [3]float32

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
		State:           StateDisconnected,
		ForwardSpeed:    200,
		BackSpeed:       200,
		SideSpeed:       350,
		UpSpeed:         200,
		YawSpeed:        140,
		PitchSpeed:      150,
		AngleSpeedKey:   1.5,
		MoveSpeedKey:    2.0,
		AlwaysRun:       true,
		FreeLook:        true,
		MaxPitch:        defaultMaxPitch,
		MinPitch:        defaultMinPitch,
		WheelPitch:      defaultWheelPitch,
		EntityBaselines: make(map[int]inet.EntityState),
		Entities:        make(map[int]inet.EntityState),
		Frags:           make(map[int]int),
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
	c.Paused = false
	c.CenterPrint = ""
	c.FixAngle = false
	c.MoveMessages = 0
	c.InImpulse = 0
	c.PendingCmd = UserCmd{}
	c.Cmd = UserCmd{}
	c.Stats = [32]int{}
	c.StatsF = [32]float32{}
	c.Items = 0
	c.DamageTaken = 0
	c.DamageSaved = 0
	c.DamageOrigin = [3]float32{}
	c.OnGround = false
	c.InWater = false
	c.SoundEvents = nil
	c.ParticleEvents = nil
	c.TempEntities = nil
	c.StaticEntities = nil
	c.StaticSounds = nil
	if c.Frags == nil {
		c.Frags = make(map[int]int)
	} else {
		clear(c.Frags)
	}
	if c.EntityBaselines == nil {
		c.EntityBaselines = make(map[int]inet.EntityState)
	} else {
		clear(c.EntityBaselines)
	}
	if c.Entities == nil {
		c.Entities = make(map[int]inet.EntityState)
	} else {
		clear(c.Entities)
	}
}

func (c *Client) ConsumeParticleEvents() []ParticleEvent {
	if c == nil || len(c.ParticleEvents) == 0 {
		return nil
	}
	events := c.ParticleEvents
	c.ParticleEvents = nil
	return events
}

func (c *Client) ConsumeTempEntities() []TempEntityEvent {
	if c == nil || len(c.TempEntities) == 0 {
		return nil
	}
	events := c.TempEntities
	c.TempEntities = nil
	return events
}

func (c *Client) ConsumeStuffCommands() string {
	if c == nil || c.StuffCmdBuf == "" {
		return ""
	}
	idx := strings.LastIndexByte(c.StuffCmdBuf, '\n')
	if idx < 0 {
		return ""
	}
	cmds := c.StuffCmdBuf[:idx+1]
	c.StuffCmdBuf = c.StuffCmdBuf[idx+1:]
	return cmds
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
		c.setState(StateActive)
	}
}

func (c *Client) setState(next ClientState) {
	if c.State == next {
		return
	}
	c.State = next
	log.Printf("Client state changed to %s", stateLogLabel(next))
}

func stateLogLabel(state ClientState) string {
	switch state {
	case StateDisconnected:
		return "Disconnected"
	case StateConnected:
		return "Connected"
	case StateActive:
		return "Active"
	default:
		return "Unknown"
	}
}

func (c *Client) HandleServerInfo() error {
	if c.State != StateDisconnected {
		return fmt.Errorf("serverinfo requires disconnected state, got %s", c.State)
	}
	c.setState(StateConnected)
	return nil
}

func (c *Client) HandleSignonReply(command string) error {
	if c.State != StateConnected {
		return fmt.Errorf("%s requires connected state, got %s", command, c.State)
	}

	switch command {
	case "prespawn":
		if c.Signon != 0 {
			return fmt.Errorf("prespawn requires signon 0, got %d", c.Signon)
		}
		c.Signon = 1
	case "spawn":
		if c.Signon != 1 {
			return fmt.Errorf("spawn requires signon 1, got %d", c.Signon)
		}
		c.Signon = 2
	case "begin":
		if c.Signon != 2 {
			return fmt.Errorf("begin requires signon 2, got %d", c.Signon)
		}
		c.Signon = 4
		c.setState(StateActive)
	default:
		return fmt.Errorf("unsupported signon reply %q", command)
	}

	return nil
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
