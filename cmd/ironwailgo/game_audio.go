package main

import (
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/audio"
	"github.com/darkliquid/ironwail-go/internal/bsp"
	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

func resetRuntimeSoundState() {
	g.SoundSFXByIndex = nil
	g.MenuSFXByName = nil
	g.AmbientSFX = [audio.NumAmbients]*audio.SFX{}
	g.SoundPrecacheKey = ""
	g.StaticSoundKey = ""
	g.MusicTrackKey = ""
}

func refreshRuntimeSoundCache() {
	if g.Client == nil {
		resetRuntimeSoundState()
		return
	}
	key := strings.Join(g.Client.SoundPrecache, "\x00")
	if key == g.SoundPrecacheKey {
		return
	}
	g.SoundPrecacheKey = key
	g.SoundSFXByIndex = make(map[int]*audio.SFX)
}

func resolveRuntimeSFX(soundIndex int) *audio.SFX {
	if g.Audio == nil || g.Client == nil || g.Subs == nil || g.Subs.Files == nil || soundIndex <= 0 {
		return nil
	}
	refreshRuntimeSoundCache()
	if sfx, ok := g.SoundSFXByIndex[soundIndex]; ok {
		return sfx
	}
	precacheIndex := soundIndex - 1
	if precacheIndex < 0 || precacheIndex >= len(g.Client.SoundPrecache) {
		g.SoundSFXByIndex[soundIndex] = nil
		return nil
	}
	soundName := g.Client.SoundPrecache[precacheIndex]
	if soundName == "" {
		g.SoundSFXByIndex[soundIndex] = nil
		return nil
	}
	sfx := g.Audio.PrecacheSound(soundName, func() ([]byte, error) {
		return g.Subs.Files.LoadFile("sound/" + soundName)
	})
	g.SoundSFXByIndex[soundIndex] = sfx
	return sfx
}

func resolveNamedRuntimeSFX(soundName string) *audio.SFX {
	if g.Audio == nil || g.Subs == nil || g.Subs.Files == nil || soundName == "" {
		return nil
	}
	return g.Audio.PrecacheSound(soundName, func() ([]byte, error) {
		return g.Subs.Files.LoadFile("sound/" + soundName)
	})
}

func resolveMenuSFX(name string) *audio.SFX {
	if g.Audio == nil || g.Subs == nil || g.Subs.Files == nil || name == "" {
		return nil
	}
	if g.MenuSFXByName == nil {
		g.MenuSFXByName = make(map[string]*audio.SFX)
	}
	if sfx, ok := g.MenuSFXByName[name]; ok {
		return sfx
	}
	sfx := g.Audio.PrecacheSound(name, func() ([]byte, error) {
		return g.Subs.Files.LoadFile("sound/" + name)
	})
	g.MenuSFXByName[name] = sfx
	return sfx
}

func resolveAmbientSFX(name string) *audio.SFX {
	if name == "" {
		return nil
	}
	if g.Audio == nil || g.Subs == nil || g.Subs.Files == nil {
		return nil
	}
	return g.Audio.PrecacheSound(name, func() ([]byte, error) {
		return g.Subs.Files.LoadFile("sound/" + name)
	})
}

func ensureRuntimeAmbientSFX() {
	if g.Audio == nil {
		g.AmbientSFX = [audio.NumAmbients]*audio.SFX{}
		return
	}

	if g.AmbientSFX[0] == nil {
		if sfx := resolveAmbientSFX("ambience/water1.wav"); sfx != nil {
			g.AmbientSFX[0] = sfx
		}
	}
	if g.AmbientSFX[1] == nil {
		if sfx := resolveAmbientSFX("ambience/wind2.wav"); sfx != nil {
			g.AmbientSFX[1] = sfx
		}
	}

	for i, sfx := range g.AmbientSFX {
		if sfx != nil {
			g.Audio.SetAmbientSound(i, sfx)
		}
	}
}

func runtimeUnderwaterIntensity(contents int32) float32 {
	switch contents {
	case bsp.ContentsWater, bsp.ContentsSlime, bsp.ContentsLava:
		return 1
	default:
		return 0
	}
}

// runtimeWaterwarpState returns the current underwater visual warp state
// based on r_waterwarp cvar, camera leaf contents, and optional menu forced-underwater.
//
// Returns:
//   - waterWarp true: r_waterwarp == 1 and camera is in liquid (or forced); use screen-space post-process.
//   - waterwarpFOV true: r_waterwarp > 1 and camera is in liquid (or forced); use FOV modulation.
//   - warpTime: the time value to use for warp animation.
//
// Mirrors C Ironwail R_SetupView() r_waterwarp logic and R_WarpScaleView() time selection.
func runtimeWaterwarpState() (waterWarp, waterwarpFOV bool, warpTime float32) {
	wwCvar := cvar.Get(renderer.CvarRWaterwarp)
	if wwCvar == nil || wwCvar.Float32() == 0 {
		return false, false, 0
	}
	wwValue := wwCvar.Float32()

	// Forced-underwater from menu preview (mirrors C M_ForcedUnderwater()).
	forced := g.Menu != nil && g.Menu.ForcedUnderwater()

	// Camera in liquid leaf (from most recent syncRuntimeAmbientAudio call).
	active := g.CameraInLiquid || forced

	if !active {
		return false, false, 0
	}

	var t float32
	if forced && g.Host != nil {
		t = float32(g.Host.RealTime())
	} else if g.Client != nil {
		t = float32(g.Client.Time)
	}

	if wwValue > 1.0 {
		return false, true, t
	}
	return true, false, t
}

func pointInTreeLeaf(tree *bsp.Tree, point [3]float32) (bsp.TreeLeaf, bool) {
	if tree == nil || len(tree.Nodes) == 0 || len(tree.Planes) == 0 || len(tree.Leafs) == 0 {
		return bsp.TreeLeaf{}, false
	}

	nodeIndex := 0
	for {
		if nodeIndex < 0 || nodeIndex >= len(tree.Nodes) {
			return bsp.TreeLeaf{}, false
		}
		node := tree.Nodes[nodeIndex]
		if int(node.PlaneNum) < 0 || int(node.PlaneNum) >= len(tree.Planes) {
			return bsp.TreeLeaf{}, false
		}
		plane := tree.Planes[node.PlaneNum]
		dist := point[0]*plane.Normal[0] + point[1]*plane.Normal[1] + point[2]*plane.Normal[2] - plane.Dist
		side := 0
		if dist < 0 {
			side = 1
		}

		child := node.Children[side]
		if child.IsLeaf {
			if child.Index < 0 || child.Index >= len(tree.Leafs) {
				return bsp.TreeLeaf{}, false
			}
			return tree.Leafs[child.Index], true
		}
		nodeIndex = child.Index
	}
}

func syncRuntimeAmbientAudio(viewOrigin [3]float32, frameTime float32) {
	if g.Audio == nil {
		return
	}

	ensureRuntimeAmbientSFX()

	var (
		ambientLevels [audio.NumAmbients]uint8
		hasLeaf       bool
		underwater    float32
	)
	if g.Client != nil && g.Client.State == cl.StateActive && g.Server != nil && g.Server.WorldTree != nil {
		if leaf, ok := pointInTreeLeaf(g.Server.WorldTree, viewOrigin); ok {
			hasLeaf = true
			ambientLevels[0] = leaf.AmbientLevel[bsp.AmbientWater]
			ambientLevels[1] = leaf.AmbientLevel[bsp.AmbientSky]
			underwater = runtimeUnderwaterIntensity(leaf.Contents)
			// Track liquid-leaf state for visual waterwarp (r_waterwarp) and
			// contents color shift (v_blend).
			g.CameraInLiquid = underwater > 0
			g.CameraLeafContents = leaf.Contents
		} else {
			g.CameraInLiquid = false
			g.CameraLeafContents = bsp.ContentsEmpty
		}
	} else {
		g.CameraInLiquid = false
		g.CameraLeafContents = bsp.ContentsEmpty
	}

	g.Audio.UpdateAmbientSounds(frameTime, hasLeaf, ambientLevels, underwater)
}

func playMenuSound(name string) {
	sfx := resolveMenuSFX(name)
	if sfx == nil {
		return
	}
	g.Audio.StartSound(0, 0, sfx, [3]float32{}, [3]float32{}, 1, 0)
}

func applySVolume() {
	if g.Audio == nil {
		return
	}
	vol := 0.7
	if cv := cvar.Get("s_volume"); cv != nil {
		vol = cv.Float
	}
	g.Audio.SetVolume(vol)
}

func buildRuntimeStaticSoundKey(c *cl.Client) string {
	if c == nil {
		return ""
	}
	var b strings.Builder
	b.Grow(64 + len(c.SoundPrecache)*16 + len(c.StaticSounds)*48)
	fmt.Fprintf(&b, "%p", c)
	b.WriteByte('\x1f')
	b.WriteString(strconv.Itoa(int(c.State)))
	b.WriteByte('\x1f')
	b.WriteString(g.SoundPrecacheKey)
	for _, snd := range c.StaticSounds {
		b.WriteByte('\x1f')
		b.WriteString(strconv.Itoa(snd.SoundIndex))
		b.WriteByte('\x1e')
		b.WriteString(strconv.Itoa(snd.Volume))
		b.WriteByte('\x1e')
		b.WriteString(strconv.FormatUint(uint64(math.Float32bits(snd.Attenuation)), 16))
		for i := 0; i < 3; i++ {
			b.WriteByte('\x1e')
			b.WriteString(strconv.FormatUint(uint64(math.Float32bits(snd.Origin[i])), 16))
		}
	}
	return b.String()
}

func syncRuntimeStaticSounds() {
	if g.Audio == nil {
		g.StaticSoundKey = ""
		return
	}
	if g.Client == nil || g.Client.State != cl.StateActive {
		if g.StaticSoundKey != "" {
			g.Audio.ClearStaticSounds()
			g.StaticSoundKey = ""
		}
		return
	}

	refreshRuntimeSoundCache()
	key := buildRuntimeStaticSoundKey(g.Client)
	if key == g.StaticSoundKey {
		return
	}

	g.Audio.ClearStaticSounds()
	for _, staticSound := range g.Client.StaticSounds {
		sfx := resolveRuntimeSFX(staticSound.SoundIndex)
		if sfx == nil {
			continue
		}
		g.Audio.StartStaticSound(
			sfx,
			staticSound.Origin,
			[3]float32{}, // Static sounds have no velocity
			float32(staticSound.Volume)/255.0,
			staticSound.Attenuation,
		)
	}
	g.StaticSoundKey = key
}

func runtimeMusicSelection() (track, loopTrack int) {
	if g.Host != nil {
		if demo := g.Host.DemoState(); demo != nil && demo.Playback {
			if g.Client != nil && g.Client.CDTrack != 0 {
				track = g.Client.CDTrack
				loopTrack = g.Client.LoopTrack
			} else if demo.CDTrack != 0 {
				track = demo.CDTrack
				loopTrack = demo.CDTrack
			}
			if track != 0 && loopTrack == 0 {
				loopTrack = track
			}
			return track, loopTrack
		}
	}
	if g.Client == nil {
		return 0, 0
	}
	track = g.Client.CDTrack
	loopTrack = g.Client.LoopTrack
	if track != 0 && loopTrack == 0 {
		loopTrack = track
	}
	return track, loopTrack
}

func syncRuntimeMusic() {
	track, loopTrack := runtimeMusicSelection()
	key := fmt.Sprintf("%d/%d", track, loopTrack)

	if g.Audio == nil {
		g.MusicTrackKey = ""
		return
	}
	if key == g.MusicTrackKey {
		return
	}
	g.MusicTrackKey = key
	if track == 0 {
		g.Audio.StopMusic()
		return
	}
	if g.Subs == nil || g.Subs.Files == nil {
		g.Audio.StopMusic()
		slog.Warn("cannot play cd track without filesystem", "track", track)
		return
	}
	if err := g.Audio.PlayCDTrack(track, loopTrack, func(name string) ([]byte, error) {
		return g.Subs.Files.LoadFile(name)
	}, func(candidates []string) (string, []byte, error) {
		return g.Subs.Files.LoadFirstAvailable(candidates)
	}); err != nil {
		if strings.Contains(err.Error(), "none of the files were found: music/track") {
			slog.Debug("cd track not available", "track", track, "loop", loopTrack)
			return
		}
		slog.Warn("failed to play cd track", "track", track, "loop", loopTrack, "error", err)
	}
}

func processRuntimeAudioEvents(viewOrigin [3]float32, transientEvents cl.TransientEvents) {
	if g.Audio == nil {
		return
	}
	soundEvents := transientEvents.SoundEvents
	stopEvents := transientEvents.StopSoundEvents
	for _, stopEvent := range stopEvents {
		g.Audio.StopSound(stopEvent.Entity, stopEvent.Channel)
	}
	for _, soundEvent := range soundEvents {
		sfx := resolveRuntimeSFX(soundEvent.SoundIndex)
		if soundEvent.SoundName != "" {
			sfx = resolveNamedRuntimeSFX(soundEvent.SoundName)
		}
		if sfx == nil {
			continue
		}
		origin := soundEvent.Origin
		entNum := soundEvent.Entity
		entChannel := soundEvent.Channel
		attenuation := soundEvent.Attenuation
		if soundEvent.Local {
			origin = viewOrigin
			attenuation = 0
			if g.Client.ViewEntity != 0 {
				entNum = g.Client.ViewEntity
			}
		}
		g.Audio.StartSound(
			entNum,
			entChannel,
			sfx,
			origin,
			[3]float32{}, // Velocity unknown for most entities
			float32(soundEvent.Volume)/255.0,
			attenuation,
		)
	}
}
