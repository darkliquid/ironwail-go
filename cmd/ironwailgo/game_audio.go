package main

import (
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"

	"github.com/ironwail/ironwail-go/internal/audio"
	"github.com/ironwail/ironwail-go/internal/bsp"
	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/renderer"
)

func resetRuntimeSoundState() {
	soundSFXByIndex = nil
	menuSFXByName = nil
	ambientSFX = [audio.NumAmbients]*audio.SFX{}
	soundPrecacheKey = ""
	staticSoundKey = ""
	musicTrackKey = ""
}

func refreshRuntimeSoundCache() {
	if gameClient == nil {
		resetRuntimeSoundState()
		return
	}
	key := strings.Join(gameClient.SoundPrecache, "\x00")
	if key == soundPrecacheKey {
		return
	}
	soundPrecacheKey = key
	soundSFXByIndex = make(map[int]*audio.SFX)
}

func resolveRuntimeSFX(soundIndex int) *audio.SFX {
	if gameAudio == nil || gameClient == nil || gameSubs == nil || gameSubs.Files == nil || soundIndex <= 0 {
		return nil
	}
	refreshRuntimeSoundCache()
	if sfx, ok := soundSFXByIndex[soundIndex]; ok {
		return sfx
	}
	precacheIndex := soundIndex - 1
	if precacheIndex < 0 || precacheIndex >= len(gameClient.SoundPrecache) {
		soundSFXByIndex[soundIndex] = nil
		return nil
	}
	soundName := gameClient.SoundPrecache[precacheIndex]
	if soundName == "" {
		soundSFXByIndex[soundIndex] = nil
		return nil
	}
	sfx := gameAudio.PrecacheSound(soundName, func() ([]byte, error) {
		return gameSubs.Files.LoadFile("sound/" + soundName)
	})
	soundSFXByIndex[soundIndex] = sfx
	return sfx
}

func resolveMenuSFX(name string) *audio.SFX {
	if gameAudio == nil || gameSubs == nil || gameSubs.Files == nil || name == "" {
		return nil
	}
	if menuSFXByName == nil {
		menuSFXByName = make(map[string]*audio.SFX)
	}
	if sfx, ok := menuSFXByName[name]; ok {
		return sfx
	}
	sfx := gameAudio.PrecacheSound(name, func() ([]byte, error) {
		return gameSubs.Files.LoadFile("sound/" + name)
	})
	menuSFXByName[name] = sfx
	return sfx
}

func resolveAmbientSFX(name string) *audio.SFX {
	if name == "" {
		return nil
	}
	if gameAudio == nil || gameSubs == nil || gameSubs.Files == nil {
		return nil
	}
	return gameAudio.PrecacheSound(name, func() ([]byte, error) {
		return gameSubs.Files.LoadFile("sound/" + name)
	})
}

func ensureRuntimeAmbientSFX() {
	if gameAudio == nil {
		ambientSFX = [audio.NumAmbients]*audio.SFX{}
		return
	}

	if ambientSFX[0] == nil {
		if sfx := resolveAmbientSFX("ambience/water1.wav"); sfx != nil {
			ambientSFX[0] = sfx
		}
	}
	if ambientSFX[1] == nil {
		if sfx := resolveAmbientSFX("ambience/wind2.wav"); sfx != nil {
			ambientSFX[1] = sfx
		}
	}

	for i, sfx := range ambientSFX {
		if sfx != nil {
			gameAudio.SetAmbientSound(i, sfx)
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
	forced := gameMenu != nil && gameMenu.ForcedUnderwater()

	// Camera in liquid leaf (from most recent syncRuntimeAmbientAudio call).
	active := runtimeCameraInLiquid || forced

	if !active {
		return false, false, 0
	}

	// Time: use realtime for forced preview so it animates even while game is paused.
	// In Go we use cl.time for both (no separate realtime equivalent exposed here).
	// This is a minor divergence; note it for doc purposes.
	var t float32
	if gameClient != nil {
		t = float32(gameClient.Time)
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
	if gameAudio == nil {
		return
	}

	ensureRuntimeAmbientSFX()

	var (
		ambientLevels [audio.NumAmbients]uint8
		hasLeaf       bool
		underwater    float32
	)
	if gameClient != nil && gameClient.State == cl.StateActive && gameServer != nil && gameServer.WorldTree != nil {
		if leaf, ok := pointInTreeLeaf(gameServer.WorldTree, viewOrigin); ok {
			hasLeaf = true
			ambientLevels[0] = leaf.AmbientLevel[bsp.AmbientWater]
			ambientLevels[1] = leaf.AmbientLevel[bsp.AmbientSky]
			underwater = runtimeUnderwaterIntensity(leaf.Contents)
			// Track liquid-leaf state for visual waterwarp (r_waterwarp) and
			// contents color shift (v_blend).
			runtimeCameraInLiquid = underwater > 0
			runtimeCameraLeafContents = leaf.Contents
		} else {
			runtimeCameraInLiquid = false
			runtimeCameraLeafContents = bsp.ContentsEmpty
		}
	} else {
		runtimeCameraInLiquid = false
		runtimeCameraLeafContents = bsp.ContentsEmpty
	}

	gameAudio.UpdateAmbientSounds(frameTime, hasLeaf, ambientLevels, underwater)
}

func playMenuSound(name string) {
	sfx := resolveMenuSFX(name)
	if sfx == nil {
		return
	}
	gameAudio.StartSound(0, 0, sfx, [3]float32{}, [3]float32{}, 1, 0)
}

func applySVolume() {
	if gameAudio == nil {
		return
	}
	vol := 0.7
	if cv := cvar.Get("s_volume"); cv != nil {
		vol = cv.Float
	}
	gameAudio.SetVolume(vol)
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
	b.WriteString(soundPrecacheKey)
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
	if gameAudio == nil {
		staticSoundKey = ""
		return
	}
	if gameClient == nil || gameClient.State != cl.StateActive {
		if staticSoundKey != "" {
			gameAudio.ClearStaticSounds()
			staticSoundKey = ""
		}
		return
	}

	refreshRuntimeSoundCache()
	key := buildRuntimeStaticSoundKey(gameClient)
	if key == staticSoundKey {
		return
	}

	gameAudio.ClearStaticSounds()
	for _, staticSound := range gameClient.StaticSounds {
		sfx := resolveRuntimeSFX(staticSound.SoundIndex)
		if sfx == nil {
			continue
		}
		gameAudio.StartStaticSound(
			sfx,
			staticSound.Origin,
			[3]float32{}, // Static sounds have no velocity
			float32(staticSound.Volume)/255.0,
			staticSound.Attenuation,
		)
	}
	staticSoundKey = key
}

func runtimeMusicSelection() (track, loopTrack int) {
	if gameHost != nil {
		if demo := gameHost.DemoState(); demo != nil && demo.Playback {
			if gameClient != nil && gameClient.CDTrack != 0 {
				track = gameClient.CDTrack
				loopTrack = gameClient.LoopTrack
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
	if gameClient == nil {
		return 0, 0
	}
	track = gameClient.CDTrack
	loopTrack = gameClient.LoopTrack
	if track != 0 && loopTrack == 0 {
		loopTrack = track
	}
	return track, loopTrack
}

func syncRuntimeMusic() {
	track, loopTrack := runtimeMusicSelection()
	key := fmt.Sprintf("%d/%d", track, loopTrack)

	if gameAudio == nil {
		musicTrackKey = ""
		return
	}
	if key == musicTrackKey {
		return
	}
	musicTrackKey = key
	if track == 0 {
		gameAudio.StopMusic()
		return
	}
	if gameSubs == nil || gameSubs.Files == nil {
		gameAudio.StopMusic()
		slog.Warn("cannot play cd track without filesystem", "track", track)
		return
	}
	if err := gameAudio.PlayCDTrack(track, loopTrack, func(name string) ([]byte, error) {
		return gameSubs.Files.LoadFile(name)
	}, func(candidates []string) (string, []byte, error) {
		return gameSubs.Files.LoadFirstAvailable(candidates)
	}); err != nil {
		slog.Warn("failed to play cd track", "track", track, "loop", loopTrack, "error", err)
	}
}

func processRuntimeAudioEvents(viewOrigin [3]float32, transientEvents cl.TransientEvents) {
	if gameAudio == nil {
		return
	}
	soundEvents := transientEvents.SoundEvents
	stopEvents := transientEvents.StopSoundEvents
	for _, stopEvent := range stopEvents {
		gameAudio.StopSound(stopEvent.Entity, stopEvent.Channel)
	}
	for _, soundEvent := range soundEvents {
		sfx := resolveRuntimeSFX(soundEvent.SoundIndex)
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
			if gameClient.ViewEntity != 0 {
				entNum = gameClient.ViewEntity
			}
		}
		gameAudio.StartSound(
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
