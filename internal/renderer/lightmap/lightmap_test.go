package lightmap

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/renderer/world"
)

func TestCompositePageRGBA_DefaultsUntouchedTexelsToBlack(t *testing.T) {
	page := &world.WorldLightmapPage{Width: 2, Height: 1}
	got := CompositePageRGBA(page, DefaultStyleValues())
	want := []byte{
		0, 0, 0, 255,
		0, 0, 0, 255,
	}
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %d, want %d (full rgba=%v)", i, got[i], want[i], got)
		}
	}
}

func TestCompositePageRGBA_CompositesSurfaceIntoBlackPage(t *testing.T) {
	page := &world.WorldLightmapPage{
		Width:  2,
		Height: 1,
		Surfaces: []world.WorldLightmapSurface{{
			X:       0,
			Y:       0,
			Width:   1,
			Height:  1,
			Styles:  [bsp.MaxLightmaps]uint8{0, 255, 255, 255},
			Samples: []byte{128, 64, 32},
		}},
	}
	got := CompositePageRGBA(page, DefaultStyleValues())
	want := []byte{
		128, 64, 32, 255,
		0, 0, 0, 255,
	}
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %d, want %d (full rgba=%v)", i, got[i], want[i], got)
		}
	}
}

func TestCompositePageRGBA_ReusesCachedBufferForDirtySurfaces(t *testing.T) {
	page := &world.WorldLightmapPage{
		Width:  2,
		Height: 1,
		Surfaces: []world.WorldLightmapSurface{
			{
				X:       0,
				Y:       0,
				Width:   1,
				Height:  1,
				Styles:  [bsp.MaxLightmaps]uint8{0, 255, 255, 255},
				Samples: []byte{200, 0, 0},
			},
			{
				X:       1,
				Y:       0,
				Width:   1,
				Height:  1,
				Styles:  [bsp.MaxLightmaps]uint8{1, 255, 255, 255},
				Samples: []byte{0, 200, 0},
			},
		},
	}
	values := DefaultStyleValues()
	values[1] = 1
	first := CompositePageRGBA(page, values)
	if len(first) == 0 {
		t.Fatal("expected initial lightmap RGBA")
	}
	firstPixel := first[0]
	cleanPixel := first[4:8]
	cachePtr := &first[0]

	values[0] = 0.5
	page.Dirty = true
	page.Surfaces[0].Dirty = true
	second := CompositePageRGBA(page, values)
	if len(second) == 0 {
		t.Fatal("expected recomposited lightmap RGBA")
	}
	if &second[0] != cachePtr {
		t.Fatal("expected dirty recomposite to reuse cached lightmap buffer")
	}
	if second[0] >= firstPixel {
		t.Fatalf("expected dirty surface to darken, got %d want < %d", second[0], firstPixel)
	}
	for i := range cleanPixel {
		if second[4+i] != cleanPixel[i] {
			t.Fatalf("clean surface byte %d changed: got %d want %d", i, second[4+i], cleanPixel[i])
		}
	}
}

func TestExtractRegionRGBAReusesScratchBuffer(t *testing.T) {
	rgba := []byte{
		1, 2, 3, 4, 5, 6, 7, 8,
		9, 10, 11, 12, 13, 14, 15, 16,
	}
	first := ExtractRegionRGBA(nil, rgba, 2, 1, 0, 1, 2)
	if len(first) != 8 {
		t.Fatalf("len(first) = %d, want 8", len(first))
	}
	if first[0] != 5 || first[4] != 13 {
		t.Fatalf("unexpected region bytes: %v", first)
	}
	ptr := &first[0]
	second := ExtractRegionRGBA(first, rgba, 2, 1, 0, 1, 2)
	if len(second) != 8 {
		t.Fatalf("len(second) = %d, want 8", len(second))
	}
	if &second[0] != ptr {
		t.Fatal("expected region extraction to reuse scratch buffer")
	}
}

func TestDirtyBoundsEmptyPage(t *testing.T) {
	page := world.WorldLightmapPage{Width: 4, Height: 4}
	x, y, w, h := DirtyBounds(page)
	if x != 0 || y != 0 || w != 0 || h != 0 {
		t.Fatalf("DirtyBounds(empty) = (%d,%d,%d,%d), want (0,0,0,0)", x, y, w, h)
	}
}

func TestDirtyBoundsIncludesOnlyDirtySurfaces(t *testing.T) {
	page := world.WorldLightmapPage{
		Width:  8,
		Height: 8,
		Surfaces: []world.WorldLightmapSurface{
			{X: 1, Y: 1, Width: 2, Height: 2, Dirty: true},
			{X: 5, Y: 5, Width: 2, Height: 2, Dirty: false},
			{X: 3, Y: 3, Width: 2, Height: 2, Dirty: true},
		},
	}
	x, y, w, h := DirtyBounds(page)
	if x != 1 || y != 1 || w != 4 || h != 4 {
		t.Fatalf("DirtyBounds = (%d,%d,%d,%d), want (1,1,4,4)", x, y, w, h)
	}
}

func TestStackPagesPadsAndReplicatesEdges(t *testing.T) {
	pageSize := 2
	pages := []world.WorldLightmapPage{
		{
			Width:  2,
			Height: 1,
			Surfaces: []world.WorldLightmapSurface{{
				X:       0,
				Y:       0,
				Width:   2,
				Height:  1,
				Styles:  [bsp.MaxLightmaps]uint8{0, 255, 255, 255},
				Samples: []byte{1, 2, 3, 4, 5, 6},
			}},
		},
	}
	got := StackPages(pages, DefaultStyleValues(), pageSize)
	// 2 pages worth of width, height = (1 + 2) * 1 = 3 rows, RGBA.
	if len(got) != 2*3*4 {
		t.Fatalf("len(got) = %d, want %d", len(got), 2*3*4)
	}
	// Padding row above content (row 0) replicates content top edge.
	wantTop := []byte{1, 2, 3, 255, 4, 5, 6, 255}
	for i := range wantTop {
		if got[i] != wantTop[i] {
			t.Fatalf("top pad byte %d = %d, want %d", i, got[i], wantTop[i])
		}
	}
	// Content row (row 1).
	wantContent := []byte{1, 2, 3, 255, 4, 5, 6, 255}
	contentStart := 8
	for i := range wantContent {
		if got[contentStart+i] != wantContent[i] {
			t.Fatalf("content byte %d = %d, want %d", i, got[contentStart+i], wantContent[i])
		}
	}
	// Padding row below (row 2) replicates content bottom edge.
	for i := range wantTop {
		if got[16+i] != wantTop[i] {
			t.Fatalf("bottom pad byte %d = %d, want %d", i, got[16+i], wantTop[i])
		}
	}
}

func TestStylesChangedMarksDirtyIndexes(t *testing.T) {
	var old, new_ [256]float32
	old[0] = 1
	old[3] = 0.5
	new_[0] = 1
	new_[3] = 0.25
	changed := StylesChanged(old, new_)
	if changed[0] {
		t.Fatal("style 0 unchanged but marked dirty")
	}
	if !changed[3] {
		t.Fatal("style 3 changed but not marked dirty")
	}
	if changed[255] {
		t.Fatal("style 255 spuriously dirty")
	}
	if !AnyStyleChanged(changed) {
		t.Fatal("expected anyStyleChanged true")
	}
}

func TestMarkDirtyLightmapPagesFlagsAffectedSurfaces(t *testing.T) {
	var changed [256]bool
	changed[1] = true
	pages := []world.WorldLightmapPage{{
		Width:  4,
		Height: 4,
		Surfaces: []world.WorldLightmapSurface{
			{Styles: [bsp.MaxLightmaps]uint8{0, 255, 255, 255}},
			{Styles: [bsp.MaxLightmaps]uint8{1, 255, 255, 255}},
			{Styles: [bsp.MaxLightmaps]uint8{255, 255, 255, 255}},
		},
	}}
	MarkDirtyLightmapPages(pages, changed)
	if !pages[0].Dirty {
		t.Fatal("page not dirty")
	}
	if pages[0].Surfaces[0].Dirty {
		t.Fatal("surface 0 (style 0) should not be dirty")
	}
	if !pages[0].Surfaces[1].Dirty {
		t.Fatal("surface 1 (style 1) should be dirty")
	}
	if pages[0].Surfaces[2].Dirty {
		t.Fatal("surface 2 (style 255) should not be dirty")
	}
}

func TestCompositeSurfaceRGBA_SingleStyleScaleClamps(t *testing.T) {
	// Fast path 2: single style with scale != 1.0 clamps to 255.
	values := DefaultStyleValues()
	values[0] = 2.0
	rgba := make([]byte, 4)
	surface := world.WorldLightmapSurface{
		X: 0, Y: 0, Width: 1, Height: 1,
		Styles:  [bsp.MaxLightmaps]uint8{0, 255, 255, 255},
		Samples: []byte{200, 100, 50}, // * 2.0 -> clamped
	}
	CompositeSurfaceRGBA(rgba, 1, surface, values)
	if rgba[0] != 255 || rgba[1] != 200 || rgba[2] != 100 {
		t.Fatalf("clamped rgba = %v, want (255,200,100)", rgba[:3])
	}
	if rgba[3] != 255 {
		t.Fatalf("alpha = %d, want 255", rgba[3])
	}
}

func TestCompositeSurfaceRGBA_DefaultStyleOnlyScaled(t *testing.T) {
	// Only styles present in the surface's Styles array scale; an unlisted
	// style has zero effect. Style 0 defaults to scale 1 (values[0]=1).
	values := DefaultStyleValues()
	values[2] = 10 // unused style must not affect output
	rgba := make([]byte, 8)
	surface := world.WorldLightmapSurface{
		X: 0, Y: 0, Width: 2, Height: 1,
		Styles:  [bsp.MaxLightmaps]uint8{0, 255, 255, 255},
		Samples: []byte{10, 20, 30, 40, 50, 60},
	}
	CompositeSurfaceRGBA(rgba, 2, surface, values)
	if rgba[0] != 10 || rgba[4] != 40 {
		t.Fatalf("style-0 scale should be identity, got %v", rgba[:6])
	}
}

func TestCompositeSurfaceRGBA_TwoStylesAdd(t *testing.T) {
	// Fast path 3: two styles sum their scaled samples, clamped at 255.
	values := DefaultStyleValues()
	values[1] = 0.5
	rgba := make([]byte, 4)
	surface := world.WorldLightmapSurface{
		X: 0, Y: 0, Width: 1, Height: 1,
		Styles:  [bsp.MaxLightmaps]uint8{0, 1, 255, 255},
		Samples: []byte{100, 0, 0, 100, 0, 0}, // style0=100, style1=100
	}
	CompositeSurfaceRGBA(rgba, 1, surface, values)
	// 100*1 + 100*0.5 = 150
	if rgba[0] != 150 {
		t.Fatalf("two-style red = %d, want 150", rgba[0])
	}
	if rgba[1] != 0 || rgba[2] != 0 {
		t.Fatalf("green/blue = %d,%d, want 0,0", rgba[1], rgba[2])
	}
}

func TestCompositeSurfaceRGBA_EmptySamplesSkips(t *testing.T) {
	values := DefaultStyleValues()
	rgba := make([]byte, 4)
	surface := world.WorldLightmapSurface{
		X: 0, Y: 0, Width: 2, Height: 2, // needs 12 sample bytes for 1 style
		Styles:  [bsp.MaxLightmaps]uint8{0, 255, 255, 255},
		Samples: []byte{1, 2, 3}, // too short
	}
	CompositeSurfaceRGBA(rgba, 2, surface, values)
	if rgba[0] != 0 || rgba[1] != 0 || rgba[2] != 0 {
		t.Fatalf("short-samples surface should be skipped, got %v", rgba[:3])
	}
}

func TestRecompositeDirtySurfacesReports(t *testing.T) {
	values := DefaultStyleValues()
	page := world.WorldLightmapPage{Width: 2, Height: 1, Surfaces: []world.WorldLightmapSurface{
		{X: 0, Y: 0, Width: 1, Height: 1, Dirty: true, Styles: [bsp.MaxLightmaps]uint8{0, 255, 255, 255}, Samples: []byte{1, 2, 3}},
		{X: 1, Y: 0, Width: 1, Height: 1, Dirty: false, Styles: [bsp.MaxLightmaps]uint8{0, 255, 255, 255}, Samples: []byte{200, 200, 200}},
	}}
	rgba := make([]byte, 8)
	if !RecompositeDirtySurfaces(rgba, page, values) {
		t.Fatal("expected recomposite to report true (one dirty surface)")
	}
	if rgba[0] != 1 || rgba[4] != 0 {
		t.Fatalf("dirty surface composited but clean untouched: rgba=%v", rgba)
	}
}
