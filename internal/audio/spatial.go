// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

// spatialize calculates the left and right volumes for a channel based on its
// position relative to the listener.
func (s *System) spatialize(ch *Channel) {
	// anything coming from the view entity will always be full volume
	if ch.EntNum != 0 && ch.EntNum == s.viewEntity {
		ch.LeftVol = ch.MasterVol
		ch.RightVol = ch.MasterVol
		return
	}

	// calculate stereo separation and distance attenuation
	sourceVec := VectorSubtract(ch.Origin, s.listener.Origin)
	dist := VectorNormalize(&sourceVec) * ch.DistMult
	dot := DotProduct(s.listener.Right, sourceVec)

	// Doppler effect
	ch.Pitch = 1.0
	// Only apply if there's significant relative velocity and Doppler is enabled (assuming always for now)
	const soundSpeed = 1100.0 // Quake units per second
	// direction from listener to source is sourceVec (already normalized)
	vL := DotProduct(s.listener.Velocity, sourceVec)
	vS := DotProduct(ch.Velocity, sourceVec)

	if soundSpeed > vL && soundSpeed > vS {
		ch.Pitch = (soundSpeed - vL) / (soundSpeed - vS)
		if ch.Pitch < 0.5 {
			ch.Pitch = 0.5
		} else if ch.Pitch > 2.0 {
			ch.Pitch = 2.0
		}
	}

	var lscale, rscale float32
	if s.dma.Channels == 1 {
		rscale = 1.0
		lscale = 1.0
	} else {
		rscale = 1.0 + dot
		lscale = 1.0 - dot
	}

	// add in distance effect
	scale := (1.0 - dist) * rscale
	ch.RightVol = int(float32(ch.MasterVol) * scale)
	if ch.RightVol < 0 {
		ch.RightVol = 0
	}

	scale = (1.0 - dist) * lscale
	ch.LeftVol = int(float32(ch.MasterVol) * scale)
	if ch.LeftVol < 0 {
		ch.LeftVol = 0
	}
}
