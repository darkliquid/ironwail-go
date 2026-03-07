package renderer

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestNormalizeSkyboxBaseName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "", want: ""},
		{in: "sky1", want: "sky1"},
		{in: "env/sky1", want: "sky1"},
		{in: "gfx/env/sky1", want: "sky1"},
		{in: "/gfx/env/sky1", want: "sky1"},
		{in: "ENV/SKY2", want: "SKY2"},
		{in: "env\\sky3", want: "sky3"},
	}
	for _, tc := range tests {
		if got := normalizeSkyboxBaseName(tc.in); got != tc.want {
			t.Fatalf("normalizeSkyboxBaseName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSkyboxFaceSearchPathsOrder(t *testing.T) {
	paths := skyboxFaceSearchPaths("storm", "rt")
	want := []string{
		"gfx/env/stormrt.png",
		"gfx/env/stormrt.tga",
		"gfx/env/stormrt.jpg",
	}
	if len(paths) != len(want) {
		t.Fatalf("len(paths) = %d, want %d", len(paths), len(want))
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("paths[%d] = %q, want %q", i, paths[i], want[i])
		}
	}
}

func TestLoadExternalSkyboxFacesPrefersPngThenTgaThenJpg(t *testing.T) {
	pngData := encodePNG(t, 2, 2, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	tgaData := encodeTGA24(t, 2, 2, color.RGBA{R: 20, G: 30, B: 40, A: 255})
	jpgLikePNG := encodePNG(t, 2, 2, color.RGBA{R: 30, G: 40, B: 50, A: 255})

	assets := map[string][]byte{
		"gfx/env/stormrt.jpg": jpgLikePNG, // falls back to jpg for rt
		"gfx/env/stormbk.tga": tgaData,    // falls back to tga for bk
		"gfx/env/stormlf.png": pngData,    // prefers png for lf
		"gfx/env/stormft.png": pngData,
		"gfx/env/stormup.png": pngData,
		"gfx/env/stormdn.png": pngData,
	}
	loadFile := func(name string) ([]byte, error) {
		if data, ok := assets[name]; ok {
			return data, nil
		}
		return nil, errNotFound
	}

	faces, loaded := loadExternalSkyboxFaces("storm", loadFile)
	if loaded != 6 {
		t.Fatalf("loaded = %d, want 6", loaded)
	}
	if faces[0].Path != "gfx/env/stormrt.jpg" {
		t.Fatalf("rt path = %q, want jpg fallback", faces[0].Path)
	}
	if faces[1].Path != "gfx/env/stormbk.tga" {
		t.Fatalf("bk path = %q, want tga fallback", faces[1].Path)
	}
	if faces[2].Path != "gfx/env/stormlf.png" {
		t.Fatalf("lf path = %q, want png", faces[2].Path)
	}
}

func TestExternalSkyboxCubemapEligible(t *testing.T) {
	var faces [6]externalSkyboxFace
	for i := range faces {
		faces[i] = externalSkyboxFace{Width: 4, Height: 4, RGBA: make([]byte, 4*4*4)}
	}
	if !externalSkyboxCubemapEligible(faces, 6) {
		t.Fatal("expected square same-size faces to be eligible")
	}

	var partial [6]externalSkyboxFace
	partial[0] = externalSkyboxFace{Width: 4, Height: 4, RGBA: make([]byte, 4*4*4)}
	partial[1] = externalSkyboxFace{Width: 4, Height: 4, RGBA: make([]byte, 4*4*4)}
	if !externalSkyboxCubemapEligible(partial, 2) {
		t.Fatal("expected partial square same-size faces to be eligible")
	}

	faces[3].Height = 8
	if externalSkyboxCubemapEligible(faces, 6) {
		t.Fatal("expected mismatched dimensions to be ineligible")
	}
	partial[1].Width = 8
	if externalSkyboxCubemapEligible(partial, 2) {
		t.Fatal("expected partial mismatched dimensions to be ineligible")
	}
	partial[1] = externalSkyboxFace{Width: 4, Height: 2, RGBA: make([]byte, 4*2*4)}
	if externalSkyboxCubemapEligible(partial, 2) {
		t.Fatal("expected non-square faces to be ineligible")
	}
	if externalSkyboxCubemapEligible(faces, 0) {
		t.Fatal("expected zero loaded faces to be ineligible")
	}
}

func TestExternalSkyboxCubemapFaceSize(t *testing.T) {
	var faces [6]externalSkyboxFace
	faces[0] = externalSkyboxFace{Width: 8, Height: 8, RGBA: make([]byte, 8*8*4)}
	if size, ok := externalSkyboxCubemapFaceSize(faces, 1); !ok || size != 8 {
		t.Fatalf("face size = %d, ok=%v, want 8,true", size, ok)
	}
	faces[2] = externalSkyboxFace{Width: 8, Height: 8, RGBA: make([]byte, 8*8*4)}
	if size, ok := externalSkyboxCubemapFaceSize(faces, 2); !ok || size != 8 {
		t.Fatalf("face size = %d, ok=%v, want 8,true", size, ok)
	}
	faces[2] = externalSkyboxFace{Width: 16, Height: 16, RGBA: make([]byte, 16*16*4)}
	if _, ok := externalSkyboxCubemapFaceSize(faces, 2); ok {
		t.Fatal("expected mixed-size faces to be rejected")
	}
}

func TestSkyboxCubemapFaceOrderMatchesCIronwail(t *testing.T) {
	var faces [6]externalSkyboxFace
	for i, suffix := range skyboxFaceSuffixes {
		faces[i] = externalSkyboxFace{Suffix: suffix}
	}
	want := []string{"ft", "bk", "up", "dn", "rt", "lf"}
	if len(want) != len(skyboxCubemapFaceOrder) {
		t.Fatalf("len(want) = %d, want %d", len(want), len(skyboxCubemapFaceOrder))
	}
	for i, faceIndex := range skyboxCubemapFaceOrder {
		if got := faces[faceIndex].Suffix; got != want[i] {
			t.Fatalf("cubemap face %d = %q, want %q", i, got, want[i])
		}
	}
}

var errNotFound = errors.New("not found")

func encodePNG(t *testing.T, width, height int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func encodeTGA24(t *testing.T, width, height int, c color.RGBA) []byte {
	t.Helper()
	header := make([]byte, 18)
	header[2] = 2
	binary.LittleEndian.PutUint16(header[12:14], uint16(width))
	binary.LittleEndian.PutUint16(header[14:16], uint16(height))
	header[16] = 24
	header[17] = 0x20
	data := make([]byte, 0, len(header)+width*height*3)
	data = append(data, header...)
	for i := 0; i < width*height; i++ {
		data = append(data, c.B, c.G, c.R)
	}
	return data
}
