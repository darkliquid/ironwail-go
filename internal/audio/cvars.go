package audio

import (
	"github.com/ironwail/ironwail-go/internal/cvar"
)

// RegisterCVars registers the audio-related console variables that allow
// runtime adjustment of audio settings. Matches C Ironwail's audio cvars:
// volume, bgmvolume, sndspeed, snd_mixspeed, snd_filterquality, snd_waterfx.
func RegisterCVars() {
	cvar.Register("volume", "0.7", cvar.FlagArchive, "Sound effects volume (0.0-1.0)")
	cvar.Register("bgmvolume", "1.0", cvar.FlagArchive, "Background music volume (0.0-1.0)")
	cvar.Register("sndspeed", "11025", cvar.FlagNone, "Sound sample rate")
	cvar.Register("snd_mixspeed", "44100", cvar.FlagArchive, "Mixing sample rate")
	cvar.Register("snd_filterquality", "1", cvar.FlagArchive, "Sound resampling filter quality")
	cvar.Register("snd_waterfx", "1", cvar.FlagArchive, "Underwater sound effect (0=off, 1=on)")
}

// UpdateFromCVars reads audio-related cvars and applies them to the system.
// Should be called once per frame from the host loop.
func (s *System) UpdateFromCVars() {
	if !s.initialized {
		return
	}
	vol := cvar.FloatValue("volume")
	if vol < 0 {
		vol = 0
	} else if vol > 1 {
		vol = 1
	}
	s.SetVolume(vol)
}
