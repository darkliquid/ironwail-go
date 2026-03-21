package client

import "math"

import "github.com/ironwail/ironwail-go/pkg/types"

const (
	defaultCenterMove  = 0.15
	defaultCenterSpeed = 500.0
)

func (c *Client) StartPitchDrift() {
	if c == nil {
		return
	}
	if c.LastStop == c.Time {
		return
	}
	if c.NoDrift || c.PitchVel == 0 {
		c.PitchVel = defaultCenterSpeed
		c.NoDrift = false
		c.DriftMove = 0
	}
}

func (c *Client) StopPitchDrift() {
	if c == nil {
		return
	}
	c.LastStop = c.Time
	c.NoDrift = true
	c.PitchVel = 0
}

func (c *Client) DriftPitch(frametime float32, forwardMove float32) {
	if c == nil || !c.OnGround || c.DemoPlayback {
		c.DriftMove = 0
		c.PitchVel = 0
		return
	}

	if c.NoDrift {
		if absf(forwardMove) < c.ForwardSpeed {
			c.DriftMove = 0
		} else {
			c.DriftMove += frametime
		}
		if c.DriftMove > defaultCenterMove && c.LookSpring {
			c.StartPitchDrift()
		}
		return
	}

	delta := c.IdealPitch - c.ViewAngles[0]
	if delta == 0 {
		c.PitchVel = 0
		return
	}

	move := frametime * c.PitchVel
	c.PitchVel += frametime * defaultCenterSpeed
	if delta > 0 {
		if move > delta {
			c.PitchVel = 0
			move = delta
		}
		c.ViewAngles[0] += move
	} else {
		if move > -delta {
			c.PitchVel = 0
			move = -delta
		}
		c.ViewAngles[0] -= move
	}
}

func absf(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
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
	if key == -1 {
		b.Down[0] = 0
		b.Down[1] = 0
		b.State = 4
		return
	}
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

func (c *Client) KeyState(key *KButton) float32 {
	impulseDown := key.State&2 != 0
	impulseUp := key.State&4 != 0
	down := key.State&1 != 0

	var val float32
	switch {
	case impulseDown && !impulseUp:
		if down {
			val = 0.5
		}
	case impulseUp && !impulseDown:
		val = 0
	case !impulseDown && !impulseUp:
		if down {
			val = 1.0
		}
	case impulseDown && impulseUp:
		if down {
			val = 0.75
		} else {
			val = 0.25
		}
	}

	key.State &= 1
	return val
}

func (c *Client) InCutscene() bool {
	return c.FixAngle && c.ViewEntity == 0
}

func (c *Client) AdjustAngles(frametime float32) {
	if c.InCutscene() {
		return
	}

	speed := frametime
	if (c.InputSpeed.State&1 != 0) != c.AlwaysRun {
		speed *= c.AngleSpeedKey
	}

	if c.InputStrafe.State&1 == 0 {
		c.ViewAngles[1] -= speed * c.YawSpeed * c.KeyState(&c.InputRight)
		c.ViewAngles[1] += speed * c.YawSpeed * c.KeyState(&c.InputLeft)
		c.ViewAngles[1] = types.AngleMod(c.ViewAngles[1])
	}

	if c.InputKLook.State&1 != 0 {
		forward := c.KeyState(&c.InputForward)
		back := c.KeyState(&c.InputBack)
		if forward != 0 || back != 0 {
			c.StopPitchDrift()
		}
		c.ViewAngles[0] -= speed * c.PitchSpeed * forward
		c.ViewAngles[0] += speed * c.PitchSpeed * back
	}

	up := c.KeyState(&c.InputLookUp)
	down := c.KeyState(&c.InputLookDown)
	if up != 0 || down != 0 {
		c.StopPitchDrift()
	}
	c.ViewAngles[0] -= speed * c.PitchSpeed * up
	c.ViewAngles[0] += speed * c.PitchSpeed * down

	if c.ViewAngles[0] > c.MaxPitch {
		c.ViewAngles[0] = c.MaxPitch
	}
	if c.ViewAngles[0] < c.MinPitch {
		c.ViewAngles[0] = c.MinPitch
	}
	if c.ViewAngles[2] > 50 {
		c.ViewAngles[2] = 50
	}
	if c.ViewAngles[2] < -50 {
		c.ViewAngles[2] = -50
	}
}

func (c *Client) BaseMove(cmd *UserCmd) {
	*cmd = UserCmd{}

	if c.InputStrafe.State&1 != 0 {
		cmd.Side += c.SideSpeed * c.KeyState(&c.InputRight)
		cmd.Side -= c.SideSpeed * c.KeyState(&c.InputLeft)
	}

	cmd.Side += c.SideSpeed * c.KeyState(&c.InputMoveRight)
	cmd.Side -= c.SideSpeed * c.KeyState(&c.InputMoveLeft)
	cmd.Up += c.UpSpeed * c.KeyState(&c.InputUp)
	cmd.Up -= c.UpSpeed * c.KeyState(&c.InputDown)
	cmd.Side += c.MouseSideMove
	cmd.Forward += c.MouseForwardMove
	cmd.Up += c.MouseUpMove

	if c.InputKLook.State&1 == 0 {
		cmd.Forward += c.ForwardSpeed * c.KeyState(&c.InputForward)
		cmd.Forward -= c.BackSpeed * c.KeyState(&c.InputBack)
	}

	if (c.InputSpeed.State&1 != 0) != c.AlwaysRun {
		cmd.Forward *= c.MoveSpeedKey
		cmd.Side *= c.MoveSpeedKey
		cmd.Up *= c.MoveSpeedKey
	}
}

func (c *Client) AccumulateCmd(frametime float32) {
	c.AdjustAngles(frametime)
	c.MViewAngles[1] = c.MViewAngles[0]
	c.MViewAngles[0] = c.ViewAngles
	c.BaseMove(&c.PendingCmd)
	c.DriftPitch(frametime, c.PendingCmd.Forward)
	c.PendingCmd.ViewAngles = c.ViewAngles
	c.MouseSideMove = 0
	c.MouseForwardMove = 0
	c.MouseUpMove = 0

	c.PendingCmd.Buttons = 0
	if c.InputAttack.State&3 != 0 {
		c.PendingCmd.Buttons |= 1
	}
	if c.InputJump.State&3 != 0 {
		c.PendingCmd.Buttons |= 2
	}
	c.InputAttack.State &^= 2
	c.InputJump.State &^= 2

	c.PendingCmd.Impulse = c.InImpulse
	c.InImpulse = 0
	cmdMS := int(math.Round(float64(frametime * 1000)))
	if cmdMS < 0 {
		cmdMS = 0
	}
	if cmdMS > 255 {
		cmdMS = 255
	}
	c.PendingCmd.Msec = uint8(cmdMS)
}
