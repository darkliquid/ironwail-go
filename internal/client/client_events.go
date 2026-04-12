package client

import "strings"

func (c *Client) ConsumeParticleEvents() []ParticleEvent {
	if c == nil || len(c.ParticleEvents) == 0 {
		return nil
	}
	events := c.ParticleEvents
	c.ParticleEvents = nil
	return events
}

func (c *Client) ConsumeSoundEvents() []SoundEvent {
	if c == nil || len(c.SoundEvents) == 0 {
		return nil
	}
	events := c.SoundEvents
	c.SoundEvents = nil
	return events
}

func (c *Client) ConsumeStopSoundEvents() []StopSoundEvent {
	if c == nil || len(c.StopSoundEvents) == 0 {
		return nil
	}
	events := c.StopSoundEvents
	c.StopSoundEvents = nil
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

func (c *Client) ConsumeBeamSegments() []BeamSegment {
	if c == nil || len(c.BeamSegments) == 0 {
		return nil
	}
	segments := c.BeamSegments
	c.BeamSegments = nil
	return segments
}

func (c *Client) ConsumeTransientEvents() TransientEvents {
	if c == nil {
		return TransientEvents{}
	}
	return TransientEvents{
		SoundEvents:     c.ConsumeSoundEvents(),
		StopSoundEvents: c.ConsumeStopSoundEvents(),
		ParticleEvents:  c.ConsumeParticleEvents(),
		TempEntities:    c.ConsumeTempEntities(),
		BeamSegments:    c.ConsumeBeamSegments(),
	}
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
