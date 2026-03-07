package main

import (
	"math"
	"testing"

	"github.com/ironwail/ironwail-go/internal/audio"
	cl "github.com/ironwail/ironwail-go/internal/client"
	inet "github.com/ironwail/ironwail-go/internal/net"
)

func TestStartupMapArg(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "plus map", args: []string{"+map", "start"}, want: "start"},
		{name: "positional map", args: []string{"start"}, want: "start"},
		{name: "plus map wins", args: []string{"start", "+map", "e1m1"}, want: "e1m1"},
		{name: "no map", args: []string{"+skill", "2"}, want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := startupMapArg(tc.args); got != tc.want {
				t.Fatalf("startupMapArg(%v) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}

func TestRunRuntimeFrameRunsClientPrediction(t *testing.T) {
	originalHost := gameHost
	originalClient := gameClient
	t.Cleanup(func() {
		gameHost = originalHost
		gameClient = originalClient
	})

	gameHost = nil
	gameClient = cl.NewClient()
	gameClient.State = cl.StateActive
	gameClient.Entities[0] = inet.EntityState{Origin: [3]float32{100, 200, 300}}
	gameClient.PendingCmd = cl.UserCmd{
		ViewAngles: [3]float32{0, 0, 0},
		Forward:    100,
	}

	runRuntimeFrame(0.016, gameCallbacks{})

	if got := gameClient.PredictedOrigin; got[0] <= 100 {
		t.Fatalf("expected PredictPlayers to advance predicted origin, got %#v", got)
	}
}

func TestRuntimeViewStateUsesPredictedClientView(t *testing.T) {
	originalClient := gameClient
	originalServer := gameServer
	originalRenderer := gameRenderer
	t.Cleanup(func() {
		gameClient = originalClient
		gameServer = originalServer
		gameRenderer = originalRenderer
	})

	gameServer = nil
	gameRenderer = nil
	gameClient = cl.NewClient()
	gameClient.PredictedOrigin = [3]float32{64, 32, 16}
	gameClient.ViewAngles = [3]float32{10, 20, 0}

	origin, angles := runtimeViewState()
	if origin != gameClient.PredictedOrigin {
		t.Fatalf("runtimeViewState origin = %v, want %v", origin, gameClient.PredictedOrigin)
	}
	if angles != gameClient.ViewAngles {
		t.Fatalf("runtimeViewState angles = %v, want %v", angles, gameClient.ViewAngles)
	}
}

func TestRuntimeAngleVectorsYawNinety(t *testing.T) {
	forward, right, up := runtimeAngleVectors([3]float32{0, 90, 0})
	if math.Abs(float64(forward[0])) > 0.0001 || math.Abs(float64(forward[1]-1)) > 0.0001 || math.Abs(float64(forward[2])) > 0.0001 {
		t.Fatalf("forward = %v, want [0 1 0]", forward)
	}
	if math.Abs(float64(right[0]-1)) > 0.0001 || math.Abs(float64(right[1])) > 0.0001 || math.Abs(float64(right[2])) > 0.0001 {
		t.Fatalf("right = %v, want [1 0 0]", right)
	}
	if math.Abs(float64(up[0])) > 0.0001 || math.Abs(float64(up[1])) > 0.0001 || math.Abs(float64(up[2]-1)) > 0.0001 {
		t.Fatalf("up = %v, want [0 0 1]", up)
	}
}

func TestRefreshRuntimeSoundCacheResetsOnPrecacheChange(t *testing.T) {
	originalClient := gameClient
	originalMap := soundSFXByIndex
	originalKey := soundPrecacheKey
	t.Cleanup(func() {
		gameClient = originalClient
		soundSFXByIndex = originalMap
		soundPrecacheKey = originalKey
	})

	gameClient = cl.NewClient()
	gameClient.SoundPrecache = []string{"weapons/rocket1.wav"}
	soundPrecacheKey = "weapons/rocket1.wav"
	soundSFXByIndex = map[int]*audio.SFX{1: nil}

	refreshRuntimeSoundCache()
	if got := len(soundSFXByIndex); got != 1 {
		t.Fatalf("same precache unexpectedly reset cache; len = %d, want 1", got)
	}

	gameClient.SoundPrecache = []string{"weapons/shotgn2.wav"}
	refreshRuntimeSoundCache()
	if got := len(soundSFXByIndex); got != 0 {
		t.Fatalf("changed precache should reset cache; len = %d, want 0", got)
	}
}

func TestSyncRuntimeStaticSoundsTracksClientStateAndSnapshotChanges(t *testing.T) {
	originalClient := gameClient
	originalAudio := gameAudio
	originalSubs := gameSubs
	originalMap := soundSFXByIndex
	originalPrecacheKey := soundPrecacheKey
	originalStaticKey := staticSoundKey
	t.Cleanup(func() {
		gameClient = originalClient
		gameAudio = originalAudio
		gameSubs = originalSubs
		soundSFXByIndex = originalMap
		soundPrecacheKey = originalPrecacheKey
		staticSoundKey = originalStaticKey
	})

	gameSubs = nil
	gameAudio = audio.NewAudioAdapter(nil)
	gameClient = cl.NewClient()
	gameClient.State = cl.StateActive
	gameClient.SoundPrecache = []string{"ambience/drip.wav"}
	gameClient.StaticSounds = []cl.StaticSound{
		{Origin: [3]float32{10, 20, 30}, SoundIndex: 1, Volume: 255, Attenuation: 1},
	}

	syncRuntimeStaticSounds()
	firstKey := staticSoundKey
	if firstKey == "" {
		t.Fatalf("expected static sound snapshot key to be populated")
	}

	syncRuntimeStaticSounds()
	if staticSoundKey != firstKey {
		t.Fatalf("unchanged snapshot should not churn static key; got %q, want %q", staticSoundKey, firstKey)
	}

	gameClient.StaticSounds = append(gameClient.StaticSounds, cl.StaticSound{
		Origin: [3]float32{40, 50, 60}, SoundIndex: 2, Volume: 200, Attenuation: 0.5,
	})
	syncRuntimeStaticSounds()
	secondKey := staticSoundKey
	if secondKey == firstKey {
		t.Fatalf("static sound list change should rebuild snapshot key")
	}

	soundSFXByIndex = map[int]*audio.SFX{1: nil}
	gameClient.SoundPrecache = []string{"ambience/wind2.wav"}
	syncRuntimeStaticSounds()
	if got := len(soundSFXByIndex); got != 0 {
		t.Fatalf("precache change should reset runtime SFX cache before static sync; len = %d, want 0", got)
	}
	if staticSoundKey == secondKey {
		t.Fatalf("precache change should rebuild static snapshot key")
	}

	gameClient.State = cl.StateConnected
	syncRuntimeStaticSounds()
	if staticSoundKey != "" {
		t.Fatalf("non-active client state should clear static snapshot key, got %q", staticSoundKey)
	}
}
