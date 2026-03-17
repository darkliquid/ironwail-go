// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

// NullMixer is a no-op mixer for headless/test paths where a mixer dependency
// must exist but no real channel blending should happen.
type NullMixer struct {
	speed int
}

var _ MixerPipeline = (*NullMixer)(nil)

func (n *NullMixer) PaintChannels(channels []Channel, rawSamples *RawSamplesBuffer, dma *DMAInfo, paintedTime, endTime int) int {
	return endTime
}

func (n *NullMixer) SetSndSpeed(speed int) { n.speed = speed }

func (n *NullMixer) SndSpeed() int { return n.speed }
