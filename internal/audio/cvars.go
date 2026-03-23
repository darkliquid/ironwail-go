package audio

import (
	"github.com/ironwail/ironwail-go/internal/cvar"
)

// RegisterCVars registers the audio-related console variables exposed during
// canonical sound/music startup.
func RegisterCVars() {
	cvar.Register("nosound", "0", cvar.FlagNone, "Disable audio output")
	cvar.Register("volume", "0.7", cvar.FlagArchive, "Sound effects volume (0.0-1.0)")
	cvar.Register("precache", "1", cvar.FlagArchive, "Precache sounds when possible")
	cvar.Register("loadas8bit", "0", cvar.FlagNone, "Load sound effects as 8-bit")
	cvar.Register("bgmvolume", "1.0", cvar.FlagArchive, "Background music volume (0.0-1.0)")
	cvar.Register("ambient_level", "0.3", cvar.FlagArchive, "Ambient sound level scale")
	cvar.Register("ambient_fade", "100", cvar.FlagArchive, "Ambient sound fade rate")
	cvar.Register("snd_noextraupdate", "0", cvar.FlagNone, "Disable extra sound updates")
	cvar.Register("snd_show", "0", cvar.FlagNone, "Show active sound mixing stats")
	cvar.Register("_snd_mixahead", "0.1", cvar.FlagArchive, "Amount of audio to mix ahead in seconds")
	cvar.Register("sndspeed", "11025", cvar.FlagNone, "Sound sample rate")
	cvar.Register("snd_mixspeed", "44100", cvar.FlagArchive, "Mixing sample rate")
	cvar.Register("snd_filterquality", "5", cvar.FlagArchive, "Sound resampling filter quality")
	cvar.Register("snd_waterfx", "1", cvar.FlagArchive, "Underwater sound effect (0=off, 1=on)")
	cvar.Register("bgm_extmusic", "1", cvar.FlagArchive, "Allow external music playback")
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

	quality := cvar.IntValue("snd_filterquality")
	if quality < 1 || quality > 5 {
		quality = 5
	}
	if mixer, ok := s.mixer.(interface{ SetFilterQuality(int) }); ok {
		mixer.SetFilterQuality(quality)
	}
}
