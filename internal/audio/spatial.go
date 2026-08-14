// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

// spatialize calculates the left and right volumes for a channel based on its
// position relative to the listener.
func (s *System) spatialize(ch *Channel) {
	// Anything coming from the view entity is always full volume. C Quake also
	// treats entity 0 this way when cl.viewentity is 0, which is the disconnected
	// state used by forced-console/menu UI after demo playback.
	if ch.EntNum == s.viewEntity {
		ch.LeftVol = ch.MasterVol
		ch.RightVol = ch.MasterVol
		return
	}

	// calculate stereo separation and distance attenuation
	sourceVec := ch.Origin.Sub(s.listener.Origin)
	dist := sourceVec.Len() * ch.DistMult
	dot := s.listener.Right.Dot(sourceVec.Normalize())

	// Doppler effect disabled for C Ironwail parity
	ch.Pitch = 1.0

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
	if sndDebugMixerCVar != nil && snddbgLevel() >= 2 {
		SnddbgLogfAt(2, "spatialize ent=%d dist=%.3f dot=%.3f master=%d left=%d right=%d",
			ch.EntNum, dist, dot, ch.MasterVol, ch.LeftVol, ch.RightVol)
	}
}
