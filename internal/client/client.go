package client

import (
	"github.com/ironwail/ironwail-go/pkg/types"
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
	Time       float32
	ViewAngles [3]float32
	Forward    float32
	Side       float32
	Up         float32
	Buttons    int
	Impulse    int
}

type LightStyle struct {
	Length  int
	Map     [64]byte
	Average byte
	Peak    byte
}

type DLight struct {
	Origin   [3]float32
	Radius   float32
	Spawn    float32
	Die      float32
	Decay    float32
	MinLight float32
	Key      int
	Color    [3]float32
}

type Beam struct {
	Entity    int
	Model     int
	StartTime float32
	EndTime   float32
	Start     [3]float32
	End       [3]float32
}

type ColorShift struct {
	DestColor [3]int
	Percent   float32
}

type ScoreboardEntry struct {
	Name      [32]byte
	EnterTime float32
	Frags     int
	Colors    int
}

type ClientStatic struct {
	State         ClientState
	SpawnParms    [2048]byte
	DemoNum       int
	DemoLoop      bool
	Demos         [8][16]byte
	DemoRecording bool
	DemoPlayback  bool
	DemoPaused    bool
	DemoSpeed     float32
	Signon        int
}

type Client struct {
	State ClientState

	MoveMessages int
	Cmd          UserCmd
	PendingCmd   UserCmd

	Stats       [32]int
	StatsF      [32]float32
	Items       int
	ItemGetTime [32]float32

	ColorShifts     [4]ColorShift
	PrevColorShifts [4]ColorShift

	MViewAngles [2][3]float32
	ViewAngles  [3]float32

	MVelocity [2][3]float32
	Velocity  [3]float32

	PunchAngle [3]float32
	PunchTime  float32

	IdealPitch float32
	PitchVel   float32
	NoDrift    bool
	DriftMove  float32

	WheelPitch float32
	ViewHeight float32
	Crouch     float32

	Paused   bool
	OnGround bool
	InWater  bool
	FixAngle bool

	Intermission  int
	CompletedTime int

	MTime   [2]float64
	Time    float64
	OldTime float64

	LastReceivedMessage float32
	SpawnTime           float32

	MapName    [128]byte
	LevelName  [128]byte
	ViewEntity int
	MaxClients int
	GameType   int

	WorldModel  int
	NumEFrags   int
	NumEntities int
	NumStatics  int

	CDTrack   int
	LoopTrack int

	Zoom    float32
	ZoomDir float32

	Scores []*ScoreboardEntry

	ForwardSpeed float32
	BackSpeed    float32
	SideSpeed    float32
	UpSpeed      float32

	YawSpeed   float32
	PitchSpeed float32

	AngleSpeedKey float32
	MoveSpeedKey  float32

	Sensitivity float32
	MPitch      float32
	MYaw        float32
	MForward    float32
	MSide       float32

	FreeLook   float32
	LookSpring float32
	LookStrafe float32

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

	InImpulse int
}

func NewClient() *Client {
	return &Client{
		ForwardSpeed:  200,
		BackSpeed:     200,
		SideSpeed:     350,
		UpSpeed:       200,
		YawSpeed:      140,
		PitchSpeed:    150,
		AngleSpeedKey: 1.5,
		MoveSpeedKey:  2.0,
		Sensitivity:   3.0,
		MPitch:        0.022,
		MYaw:          0.022,
		MForward:      1.0,
		MSide:         0.8,
	}
}

func (c *Client) KeyDown(b *KButton, key int) {
	if key == b.Down[0] || key == b.Down[1] {
		return
	}

	if b.Down[0] == 0 {
		b.Down[0] = key
	} else if b.Down[1] == 0 {
		b.Down[1] = key
	} else {
		return
	}

	if b.State&1 != 0 {
		return
	}
	b.State |= 1 + 2
}

func (c *Client) KeyUp(b *KButton, key int) {
	if b.Down[0] == key {
		b.Down[0] = 0
	} else if b.Down[1] == key {
		b.Down[1] = 0
	} else {
		return
	}

	if b.Down[0] != 0 || b.Down[1] != 0 {
		return
	}

	if b.State&1 == 0 {
		return
	}
	b.State &^= 1
	b.State |= 4
}

func (c *Client) AdjustAngles(frametime float32) {
	speed := c.YawSpeed * frametime
	if c.InputSpeed.State&1 != 0 {
		speed *= c.AngleSpeedKey
	}

	if c.InputKLook.State&1 == 0 {
		if c.InputLeft.State&1 != 0 {
			c.ViewAngles[1] += speed
		}
		if c.InputRight.State&1 != 0 {
			c.ViewAngles[1] -= speed
		}
	}

	speed = c.PitchSpeed * frametime
	if c.InputSpeed.State&1 != 0 {
		speed *= c.AngleSpeedKey
	}

	if c.InputLookUp.State&1 != 0 {
		c.ViewAngles[0] -= speed
	}
	if c.InputLookDown.State&1 != 0 {
		c.ViewAngles[0] += speed
	}

	c.ViewAngles[0] = types.Clamp(c.ViewAngles[0], -70, 80)
	c.ViewAngles[1] = types.AngleMod(c.ViewAngles[1])
}

func (c *Client) BaseMove(cmd *UserCmd) {
	cmd.Forward = 0
	cmd.Side = 0
	cmd.Up = 0

	if c.InputKLook.State&1 != 0 {
		if c.InputForward.State&1 != 0 {
			c.ViewAngles[0] -= c.PitchSpeed * 0.016
		}
		if c.InputBack.State&1 != 0 {
			c.ViewAngles[0] += c.PitchSpeed * 0.016
		}
	} else {
		if c.InputForward.State&1 != 0 {
			cmd.Forward += c.ForwardSpeed
		}
		if c.InputBack.State&1 != 0 {
			cmd.Forward -= c.BackSpeed
		}
	}

	if c.InputMoveLeft.State&1 != 0 {
		cmd.Side -= c.SideSpeed
	}
	if c.InputMoveRight.State&1 != 0 {
		cmd.Side += c.SideSpeed
	}

	if c.InputUp.State&1 != 0 {
		cmd.Up += c.UpSpeed
	}
	if c.InputDown.State&1 != 0 {
		cmd.Up -= c.UpSpeed
	}

	if c.InputSpeed.State&1 != 0 {
		cmd.Forward *= c.MoveSpeedKey
		cmd.Side *= c.MoveSpeedKey
		cmd.Up *= c.MoveSpeedKey
	}
}

func (c *Client) AccumulateCmd(frametime float32) {
	c.AdjustAngles(frametime)

	c.PendingCmd.ViewAngles = c.ViewAngles
	c.BaseMove(&c.PendingCmd)

	c.PendingCmd.Buttons = 0
	if c.InputAttack.State&1 != 0 {
		c.PendingCmd.Buttons |= 1
	}
	if c.InputJump.State&1 != 0 {
		c.PendingCmd.Buttons |= 2
	}

	c.PendingCmd.Impulse = c.InImpulse
	c.InImpulse = 0
}
