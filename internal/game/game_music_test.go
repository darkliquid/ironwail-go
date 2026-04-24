package game

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/audio"
	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/host"
	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

func TestRuntimeMusicSelectionUsesDemoHeaderFallback(t *testing.T) {
	g := New()
	originalHost := g.Host
	originalClient := g.Client
	t.Cleanup(func() {
		g.Host = originalHost
		g.Client = originalClient
	})

	demo := cl.NewDemoState()
	demo.Playback = true
	demo.CDTrack = 5
	g.Host.SetDemoState(demo)
	g.Client = cl.NewClient()

	track, loopTrack := g.runtimeMusicSelection()
	if track != 5 || loopTrack != 5 {
		t.Fatalf("g.runtimeMusicSelection() = %d/%d, want 5/5", track, loopTrack)
	}

	g.Client.CDTrack = 2
	g.Client.LoopTrack = 3
	track, loopTrack = g.runtimeMusicSelection()
	if track != 2 || loopTrack != 3 {
		t.Fatalf("g.runtimeMusicSelection() with live client track = %d/%d, want 2/3", track, loopTrack)
	}
}

func TestRuntimeWaterwarpStateUsesRealtimeForForcedPreview(t *testing.T) {
	g := New()
	originalHost := g.Host
	originalMenu := g.Menu
	originalClient := g.Client
	t.Cleanup(func() {
		g.Host = originalHost
		g.Menu = originalMenu
		g.Client = originalClient
	})

	if g.Host.CVar.Get(renderer.CvarRWaterwarp) == nil {
		g.Host.CVar.Register(renderer.CvarRWaterwarp, "0", 0, "Underwater warp test")
	}
	g.Host.CVar.Set(renderer.CvarRWaterwarp, "2")
	t.Cleanup(func() {
		g.Host.CVar.Set(renderer.CvarRWaterwarp, "0")
	})

	g.Client = cl.NewClient()
	g.Client.Time = 12.5
	g.Menu = menu.NewManager(nil, input.NewSystem(nil), nil)
	g.Menu.ShowMenu()
	g.Menu.M_Key(input.KDownArrow)
	g.Menu.M_Key(input.KDownArrow)
	g.Menu.M_Key(input.KEnter)
	g.Menu.M_Key(input.KDownArrow)
	g.Menu.M_Key(input.KEnter)
	for i := 0; i < 6; i++ {
		g.Menu.M_Key(input.KDownArrow)
	}
	if !g.Menu.ForcedUnderwater() {
		t.Fatal("expected WATERWARP menu preview to force underwater mode")
	}

	waterWarp, waterwarpFOV, warpTime := g.runtimeWaterwarpState()
	if waterWarp {
		t.Fatalf("waterWarp = true, want false for r_waterwarp=2")
	}
	if !waterwarpFOV {
		t.Fatalf("waterwarpFOV = false, want true for r_waterwarp=2")
	}
	if warpTime != 0 {
		t.Fatalf("warpTime = %v, want host realtime 0 instead of client time %v", warpTime, g.Client.Time)
	}
}

func TestSyncRuntimeMusicLoadsTrackOnceAndStops(t *testing.T) {
	g := New()
	originalAudio := g.Audio
	originalClient := g.Client
	originalHost := g.Host
	originalSubs := g.Subs
	originalKey := g.MusicTrackKey
	t.Cleanup(func() {
		g.Audio = originalAudio
		g.Client = originalClient
		g.Host = originalHost
		g.Subs = originalSubs
		g.MusicTrackKey = originalKey
	})

	sys := audio.NewSystem()
	if err := sys.Init(audio.NewNullBackend(), 44100, false); err != nil {
		t.Fatalf("audio.Init failed: %v", err)
	}
	if err := sys.Startup(); err != nil {
		t.Fatalf("audio.Startup failed: %v", err)
	}

	g.Audio = audio.NewAudioAdapter(sys)
	g.Client = cl.NewClient()
	g.Client.CDTrack = 2
	g.Client.LoopTrack = 2
	testFS := &runtimeMusicTestFS{
		files: map[string][]byte{
			"music/track02.wav": testRuntimeMusicWAV(t, 44100, 2, 2, 64),
		},
	}
	g.Subs = &host.Subsystems{Files: testFS}

	g.syncRuntimeMusic()
	if got := sys.CurrentMusicTrack(); got != 2 {
		t.Fatalf("CurrentMusicTrack = %d, want 2", got)
	}
	if got := testFS.loads; got != 1 {
		t.Fatalf("filesystem loads = %d, want 1 after first sync", got)
	}

	g.syncRuntimeMusic()
	if got := testFS.loads; got != 1 {
		t.Fatalf("filesystem loads = %d, want no reload for unchanged request", got)
	}

	g.Client.CDTrack = 0
	g.Client.LoopTrack = 0
	g.syncRuntimeMusic()
	if got := sys.CurrentMusicTrack(); got != 0 {
		t.Fatalf("CurrentMusicTrack = %d, want 0 after stopping music", got)
	}
}
