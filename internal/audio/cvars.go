package audio

import (
	"github.com/darkliquid/ironwail-go/internal/cvar"
)

// RegisterCVars registers the audio-related console variables exposed during
// canonical sound/music startup.
func RegisterCVars(cv *cvar.CVarSystem) {
	cv.Register("nosound", "0", cvar.FlagNone, "Disable audio output")
	cv.Register("volume", "0.7", cvar.FlagArchive, "Sound effects volume (0.0-1.0)")
	cv.Register("precache", "1", cvar.FlagArchive, "Precache sounds when possible")
	cv.Register("loadas8bit", "0", cvar.FlagNone, "Load sound effects as 8-bit")
	cv.Register("bgmvolume", "1.0", cvar.FlagArchive, "Background music volume (0.0-1.0)")
	cv.Register("ambient_level", "0.3", cvar.FlagArchive, "Ambient sound level scale")
	cv.Register("ambient_fade", "100", cvar.FlagArchive, "Ambient sound fade rate")
	cv.Register("snd_noextraupdate", "0", cvar.FlagNone, "Disable extra sound updates")
	cv.Register("snd_show", "0", cvar.FlagNone, "Show active sound mixing stats")
	cv.Register("_snd_mixahead", "0.1", cvar.FlagArchive, "Amount of audio to mix ahead in seconds")
	cv.Register("sndspeed", "11025", cvar.FlagNone, "Sound sample rate")
	cv.Register("snd_mixspeed", "44100", cvar.FlagArchive, "Mixing sample rate")
	cv.Register("snd_filterquality", "5", cvar.FlagArchive, "Sound resampling filter quality")
	cv.Register("snd_waterfx", "1", cvar.FlagArchive, "Underwater sound effect (0=off, 1=on)")
	cv.Register("bgm_extmusic", "1", cvar.FlagArchive, "Allow external music playback")
}

// UpdateFromCVars reads audio-related cvars and applies them to the system.
// Should be called once per frame from the host loop.
func (s *System) UpdateFromCVars(cv *cvar.CVarSystem) {
	if !s.initialized || cv == nil {
		return
	}
	vol := cv.FloatValue("volume")
	if vol < 0 {
		vol = 0
	} else if vol > 1 {
		vol = 1
	}
	s.SetVolume(vol)

	quality := cv.IntValue("snd_filterquality")
	if quality < 1 || quality > 5 {
		quality = 5
	}
	if mixer, ok := s.mixer.(interface{ SetFilterQuality(int) }); ok {
		mixer.SetFilterQuality(quality)
	}
}
