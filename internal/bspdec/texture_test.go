package bspdec

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
)

func TestTextureNamesFromFixture(t *testing.T) {
	tree, _ := compileFixture(t, roomMap())
	names := textureNames(tree)
	want := map[string]bool{"wwall": true, "ffloor": true, "cceil": true}
	for _, n := range names {
		delete(want, n)
	}
	if len(want) != 0 {
		t.Fatalf("missing texture names: %v (got %v)", want, names)
	}
}

func TestTextureBrushesAssignsFaces(t *testing.T) {
	tree, _ := compileFixture(t, roomMap())
	d := newDecompiler(tree, Options{MergeConvex: true, GridSnap: 8, TextureFallback: "nearest"})
	brushes, err := d.decompileModel(0)
	if err != nil {
		t.Fatalf("decompileModel: %v", err)
	}
	for _, b := range brushes {
		removeRedundantPlanes(b)
	}
	d.textureBrushes(brushes)
	seen := map[string]bool{}
	for _, b := range brushes {
		for _, s := range b.Sides {
			if s.TexName == "" {
				t.Fatal("side with empty texture after texturing+fallback")
			}
			seen[s.TexName] = true
		}
	}
	// floor and wall faces must be recovered from the face lump
	if !seen["ffloor"] || !seen["wwall"] {
		t.Fatalf("expected ffloor and wwall among side textures, got %v", seen)
	}
}

func TestContentsTexture(t *testing.T) {
	cases := map[int32]string{
		bsp.ContentsWater: "*water",
		bsp.ContentsSlime: "*slime",
		bsp.ContentsLava:  "*lava",
		bsp.ContentsSky:   "sky",
		bsp.ContentsClip:  "clip",
		bsp.ContentsSolid: "",
		bsp.ContentsEmpty: "",
	}
	for c, want := range cases {
		if got := contentsTexture(c); got != want {
			t.Fatalf("contentsTexture(%d) = %q, want %q", c, got, want)
		}
	}
}

func TestFallbackNearestUsesLargestMatchedSide(t *testing.T) {
	tree, _ := compileFixture(t, roomMap())
	d := newDecompiler(tree, Options{TextureFallback: "nearest"})
	b := boxBrush(vc(0, 0, 0), vc(64, 64, 64))
	// no tree faces near this brush: every side falls back, and with no
	// matched side in the brush the policy warns and uses "clip"
	d.textureBrush(b)
	for _, s := range b.Sides {
		if s.TexName != "clip" {
			t.Fatalf("fallback texture = %q, want clip", s.TexName)
		}
	}
	if d.warnings == 0 {
		t.Fatal("expected a fallback warning")
	}
}