package client

import "github.com/ironwail/ironwail-go/pkg/types"

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
		c.ViewAngles[0] -= speed * c.PitchSpeed * c.KeyState(&c.InputForward)
		c.ViewAngles[0] += speed * c.PitchSpeed * c.KeyState(&c.InputBack)
	}

	up := c.KeyState(&c.InputLookUp)
	down := c.KeyState(&c.InputLookDown)
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
	c.PendingCmd.ViewAngles = c.ViewAngles
	c.BaseMove(&c.PendingCmd)

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
}
