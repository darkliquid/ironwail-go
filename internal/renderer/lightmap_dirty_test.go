//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import "testing"

func TestLightStylesChanged(t *testing.T) {
	var old, new_ [64]float32
	for i := range old {
		old[i] = 1.0
		new_[i] = 1.0
	}
	// No changes.
	changed := lightStylesChanged(old, new_)
	for i, c := range changed {
		if c {
			t.Errorf("expected no change at index %d", i)
		}
	}

	// Change style 5 and 10.
	new_[5] = 0.5
	new_[10] = 2.0
	changed = lightStylesChanged(old, new_)
	if !changed[5] {
		t.Error("expected change at index 5")
	}
	if !changed[10] {
		t.Error("expected change at index 10")
	}
	if changed[0] {
		t.Error("index 0 should not be changed")
	}
}

func TestMarkDirtyLightmapPages(t *testing.T) {
	pages := []WorldLightmapPage{
		{
			Width: 64, Height: 64,
			Surfaces: []WorldLightmapSurface{
				{X: 0, Y: 0, Width: 4, Height: 4, Styles: [4]uint8{0, 255, 255, 255}},
				{X: 4, Y: 0, Width: 4, Height: 4, Styles: [4]uint8{5, 255, 255, 255}},
			},
		},
		{
			Width: 64, Height: 64,
			Surfaces: []WorldLightmapSurface{
				{X: 0, Y: 0, Width: 4, Height: 4, Styles: [4]uint8{10, 255, 255, 255}},
			},
		},
	}

	// Only style 5 changed.
	var changed [64]bool
	changed[5] = true
	markDirtyLightmapPages(pages, changed)

	// Page 0: surface 0 uses style 0 (clean), surface 1 uses style 5 (dirty).
	if pages[0].Surfaces[0].Dirty {
		t.Error("surface 0 (style 0) should not be dirty")
	}
	if !pages[0].Surfaces[1].Dirty {
		t.Error("surface 1 (style 5) should be dirty")
	}
	if !pages[0].Dirty {
		t.Error("page 0 should be dirty (has dirty surface)")
	}

	// Page 1: surface uses style 10 (clean).
	if pages[1].Surfaces[0].Dirty {
		t.Error("page 1 surface (style 10) should not be dirty")
	}
	if pages[1].Dirty {
		t.Error("page 1 should not be dirty")
	}
}

func TestMarkDirtyMultiStyleSurface(t *testing.T) {
	pages := []WorldLightmapPage{
		{
			Width: 64, Height: 64,
			Surfaces: []WorldLightmapSurface{
				{X: 0, Y: 0, Width: 4, Height: 4, Styles: [4]uint8{0, 3, 7, 255}},
			},
		},
	}

	// Style 3 changed — surface references it as second style.
	var changed [64]bool
	changed[3] = true
	markDirtyLightmapPages(pages, changed)

	if !pages[0].Surfaces[0].Dirty {
		t.Error("surface with style 3 should be dirty")
	}
}

func TestClearDirtyFlags(t *testing.T) {
	pages := []WorldLightmapPage{
		{
			Width: 64, Height: 64, Dirty: true,
			Surfaces: []WorldLightmapSurface{
				{X: 0, Y: 0, Width: 4, Height: 4, Dirty: true},
				{X: 4, Y: 0, Width: 4, Height: 4, Dirty: false},
			},
		},
		{
			Width: 64, Height: 64, Dirty: false,
			Surfaces: []WorldLightmapSurface{
				{X: 0, Y: 0, Width: 4, Height: 4, Dirty: false},
			},
		},
	}

	clearDirtyFlags(pages)

	if pages[0].Dirty {
		t.Error("page 0 should be clean after clear")
	}
	if pages[0].Surfaces[0].Dirty {
		t.Error("surface 0 should be clean after clear")
	}
	if pages[1].Dirty {
		t.Error("page 1 should remain clean")
	}
}
