package game

import (
	"bytes"
	"testing"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	qimage "github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

func TestBuildCSQCDrawHooksUsesNamedPicsAndScales(t *testing.T) {
	g := New()
	originalDraw := g.Draw
	t.Cleanup(func() {
		g.Draw = originalDraw
	})

	palette := make([]byte, 768)
	g.Draw = newTestDrawManager(t, map[string]*qimage.QPic{
		"test": {Width: 2, Height: 1, Pixels: []byte{1, 2}},
	}, palette)

	dc := &csqcDrawTestContext{}
	hooks := g.buildCSQCDrawHooks(dc)

	if width, height := hooks.GetImageSize("gfx/test.lmp"); width != 2 || height != 1 {
		t.Fatalf("GetImageSize = (%v, %v), want (2, 1)", width, height)
	}
	if !hooks.IsCachedPic("gfx/test.lmp") {
		t.Fatal("IsCachedPic(gfx/test.lmp) = false after GetImageSize cache load, want true")
	}
	if hooks.IsCachedPic("gfx/missing.lmp") {
		t.Fatal("IsCachedPic(gfx/missing.lmp) = true, want false")
	}

	hooks.DrawPic(10, 20, "gfx/test.lmp", 4, 2, 1, 1, 1, 1, 0)
	if len(dc.pics) != 1 {
		t.Fatalf("DrawPic calls = %d, want 1", len(dc.pics))
	}
	got := dc.pics[0]
	if got.x != 10 || got.y != 20 {
		t.Fatalf("DrawPic coords = (%d, %d), want (10, 20)", got.x, got.y)
	}
	if got.pic == nil {
		t.Fatal("DrawPic pic = nil, want scaled pic")
	}
	if got.pic.Width != 4 || got.pic.Height != 2 {
		t.Fatalf("DrawPic size = %dx%d, want 4x2", got.pic.Width, got.pic.Height)
	}
	wantPixels := []byte{1, 1, 2, 2, 1, 1, 2, 2}
	if !bytes.Equal(got.pic.Pixels, wantPixels) {
		t.Fatalf("DrawPic pixels = %v, want %v", got.pic.Pixels, wantPixels)
	}
}

func TestBuildCSQCDrawHooksSubPicClipAndFill(t *testing.T) {
	g := New()
	originalDraw := g.Draw
	t.Cleanup(func() {
		g.Draw = originalDraw
	})

	palette := make([]byte, 768)
	palette[0], palette[1], palette[2] = 0, 0, 0
	palette[3], palette[4], palette[5] = 255, 0, 0
	palette[6], palette[7], palette[8] = 0, 255, 0
	palette[9], palette[10], palette[11] = 0, 0, 255

	g.Draw = newTestDrawManager(t, map[string]*qimage.QPic{
		"clip": {
			Width:  4,
			Height: 4,
			Pixels: []byte{
				0, 1, 2, 3,
				4, 5, 6, 7,
				8, 9, 10, 11,
				12, 13, 14, 15,
			},
		},
	}, palette)

	dc := &csqcDrawTestContext{}
	hooks := g.buildCSQCDrawHooks(dc)

	hooks.DrawSubPic(0, 0, 2, 2, "gfx/clip.lmp", 0.25, 0.25, 0.5, 0.5, 1, 1, 1, 1, 0)
	if len(dc.pics) != 1 {
		t.Fatalf("DrawSubPic calls = %d, want 1", len(dc.pics))
	}
	if got := dc.pics[0].pic; got == nil || got.Width != 2 || got.Height != 2 || !bytes.Equal(got.Pixels, []byte{5, 6, 9, 10}) {
		t.Fatalf("DrawSubPic pic = %+v, want cropped 2x2 center", dc.pics[0].pic)
	}

	hooks.SetClipArea(1, 1, 2, 2)
	hooks.DrawPic(0, 0, "gfx/clip.lmp", 4, 4, 1, 1, 1, 1, 0)
	if len(dc.pics) != 2 {
		t.Fatalf("DrawPic with clip calls = %d, want 2 total", len(dc.pics))
	}
	clipped := dc.pics[1]
	if clipped.x != 1 || clipped.y != 1 {
		t.Fatalf("clipped DrawPic coords = (%d, %d), want (1, 1)", clipped.x, clipped.y)
	}
	if clipped.pic == nil {
		t.Fatal("clipped DrawPic pic = nil, want clipped pic")
	}
	if clipped.pic.Width != 2 || clipped.pic.Height != 2 {
		t.Fatalf("clipped DrawPic size = %dx%d, want 2x2", clipped.pic.Width, clipped.pic.Height)
	}
	if want := []byte{5, 6, 9, 10}; !bytes.Equal(clipped.pic.Pixels, want) {
		t.Fatalf("clipped DrawPic pixels = %v, want %v", clipped.pic.Pixels, want)
	}

	hooks.DrawFill(0, 0, 4, 4, 0.1, 0.9, 0.1, 1, 0)
	if len(dc.fills) != 1 {
		t.Fatalf("DrawFill calls = %d, want 1", len(dc.fills))
	}
	if got := dc.fills[0]; got.x != 1 || got.y != 1 || got.w != 2 || got.h != 2 || got.color != 2 {
		t.Fatalf("clipped DrawFill = %+v, want x=1 y=1 w=2 h=2 color=2", got)
	}

	hooks.ResetClipArea()
	hooks.DrawFill(0, 0, 4, 4, 0.1, 0.9, 0.1, 1, 0)
	if len(dc.fills) != 2 {
		t.Fatalf("DrawFill after reset calls = %d, want 2", len(dc.fills))
	}
	if got := dc.fills[1]; got.x != 0 || got.y != 0 || got.w != 4 || got.h != 4 || got.color != 2 {
		t.Fatalf("reset DrawFill = %+v, want x=0 y=0 w=4 h=4 color=2", got)
	}
}

func TestBuildCSQCDrawHooksCachePicParitySemantics(t *testing.T) {
	g := New()
	originalDraw := g.Draw
	originalCSQC := g.CSQC
	t.Cleanup(func() {
		g.Draw = originalDraw
		g.CSQC = originalCSQC
	})

	palette := make([]byte, 768)
	g.Draw = newTestDrawManager(t, map[string]*qimage.QPic{
		"test": {Width: 2, Height: 1, Pixels: []byte{1, 2}},
	}, palette)
	g.CSQC = qc.NewCSQC()

	hooks := g.buildCSQCDrawHooks(&csqcDrawTestContext{})
	if hooks.IsCachedPic("gfx/test.lmp") {
		t.Fatal("IsCachedPic should be false before first non-NOLOAD cache")
	}
	if got := hooks.PrecachePic("gfx/test.lmp", int(CSQCPicFlagNoLoad)); got != "gfx/test.lmp" {
		t.Fatalf("PrecachePic(NOLOAD) = %q, want input string", got)
	}
	if hooks.IsCachedPic("gfx/test.lmp") {
		t.Fatal("IsCachedPic should remain false after NOLOAD precache query")
	}
	w, h := hooks.GetImageSize("gfx/test.lmp")
	if w != 2 || h != 1 {
		t.Fatalf("GetImageSize after cache load = (%v,%v), want (2,1)", w, h)
	}
	if !hooks.IsCachedPic("gfx/test.lmp") {
		t.Fatal("IsCachedPic should be true after AUTO cache load")
	}
	if got := hooks.PrecachePic("gfx/missing.lmp", int(CSQCPicFlagBlock)); got != "" {
		t.Fatalf("PrecachePic(BLOCK) on missing pic = %q, want empty string", got)
	}
}

func TestBuildCSQCDrawHooksWithActivityTracksActualDrawCalls(t *testing.T) {
	g := New()
	activity := &csqcDrawActivity{}
	dc := &csqcDrawTestContext{}
	hooks := g.buildCSQCDrawHooksWithActivity(dc, activity)

	hooks.DrawFill(10, 12, 20, 24, 1, 0, 0, 0, 0)
	if activity.drew {
		t.Fatal("activity should remain false for fully transparent draw")
	}

	hooks.DrawFill(10, 12, 20, 24, 1, 0, 0, 1, 0)
	if !activity.drew {
		t.Fatal("activity should become true after opaque draw")
	}
}

func TestBuildCSQCFrameStatePopulatesCSQCExtGlobals(t *testing.T) {
	g := New()
	g.Client = cl.NewClient()
	g.Client.Time = 12.5
	g.Client.MaxClients = 8
	g.Client.Intermission = 2
	g.Client.CompletedTime = 8.25
	g.Client.ViewEntity = 3
	g.Client.ViewAngles = types.Vec3{X: 1, Y: 2, Z: 3}
	g.Client.CommandSequence = 17

	state := g.buildCSQCFrameState()
	if state.Time != 12.5 {
		t.Fatalf("state.Time = %v, want 12.5", state.Time)
	}
	if state.MaxClients != 8 {
		t.Fatalf("state.MaxClients = %v, want 8", state.MaxClients)
	}
	if state.Intermission != 2 {
		t.Fatalf("state.Intermission = %v, want 2", state.Intermission)
	}
	if state.IntermissionTime != 8.25 {
		t.Fatalf("state.IntermissionTime = %v, want 8.25", state.IntermissionTime)
	}
	if state.PlayerLocalNum != 2 {
		t.Fatalf("state.PlayerLocalNum = %v, want 2", state.PlayerLocalNum)
	}
	if state.PlayerLocalEntNum != 3 {
		t.Fatalf("state.PlayerLocalEntNum = %v, want 3", state.PlayerLocalEntNum)
	}
	if state.ClientCommandFrame != 17 {
		t.Fatalf("state.ClientCommandFrame = %v, want 17", state.ClientCommandFrame)
	}
	if state.ViewAngles != (types.Vec3{X: 1, Y: 2, Z: 3}) {
		t.Fatalf("state.ViewAngles = %v, want [1 2 3]", state.ViewAngles)
	}
}
