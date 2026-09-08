package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/pkg/wad"
)

// runWad implements `qcmod wad -o out.wad <images...>`: converts PNG/TGA
// images into a Quake WAD. Each image becomes one lump: a QPic (HUD/menu
// art, parsed by wad.ParseQPic) unless it is a texture-sized image and
// -type miptex is chosen (parseable via wad.ParseMipTex / MipTex.MipLevel).
//
// Palette sourcing order: -palette file, then the built-in Quake palette
// (wad.DefaultQuakePalette). Passing --placeholder generates a minimal valid WAD.
func runWad(args []string, stdout, stderr io.Writer) int {
	out := "out.wad"
	lumpType := "auto" // auto | qpic | miptex
	palettePath := ""
	placeholder := false
	var images []string

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-o" || a == "--out" || a == "--output":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "qcmod wad: -o requires a value")
				return 2
			}
			out = args[i+1]
			i++
		case strings.HasPrefix(a, "-o="):
			out = strings.TrimPrefix(a, "-o=")
		case a == "-type":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "qcmod wad: -type requires a value")
				return 2
			}
			lumpType = args[i+1]
			i++
		case strings.HasPrefix(a, "-type="):
			lumpType = strings.TrimPrefix(a, "-type=")
		case a == "-palette" || a == "-pal":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "qcmod wad: -palette requires a value")
				return 2
			}
			palettePath = args[i+1]
			i++
		case strings.HasPrefix(a, "-palette="):
			palettePath = strings.TrimPrefix(a, "-palette=")
		case a == "--placeholder" || a == "-placeholder":
			placeholder = true
		case strings.HasPrefix(a, "-"):
			_, _ = fmt.Fprintf(stderr, "qcmod wad: unknown flag %q\n", a)
			return 2
		default:
			images = append(images, a)
		}
	}

	if placeholder {
		f, err := os.Create(out)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "qcmod wad: create %s: %v\n", out, err)
			return 1
		}
		if err := wad.WritePlaceholderWad(f); err != nil {
			_ = f.Close()
			_, _ = fmt.Fprintf(stderr, "qcmod wad: %v\n", err)
			return 1
		}
		if err := f.Close(); err != nil {
			_, _ = fmt.Fprintf(stderr, "qcmod wad: close %s: %v\n", out, err)
			return 1
		}
		_, _ = fmt.Fprintf(stdout, "Wrote placeholder WAD -> %s\n", out)
		return 0
	}

	if len(images) == 0 {
		_, _ = fmt.Fprintln(stderr, "qcmod wad: at least one image is required (or pass --placeholder)")
		return 2
	}
	switch lumpType {
	case "auto", "qpic", "miptex":
	default:
		_, _ = fmt.Fprintf(stderr, "qcmod wad: unknown -type %q (auto|qpic|miptex)\n", lumpType)
		return 2
	}

	pal, err := loadWadPalette(palettePath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod wad: %v\n", err)
		return 1
	}

	lumps := make([]wad.WadLump, 0, len(images))
	for _, path := range images {
		lump, err := buildWadLump(path, lumpType, pal)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "qcmod wad: %v\n", err)
			return 1
		}
		lumps = append(lumps, lump)
	}

	f, err := os.Create(out)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod wad: create %s: %v\n", out, err)
		return 1
	}
	if err := wad.WriteWad(f, lumps); err != nil {
		_ = f.Close()
		_, _ = fmt.Fprintf(stderr, "qcmod wad: %v\n", err)
		return 1
	}
	if err := f.Close(); err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod wad: close %s: %v\n", out, err)
		return 1
	}

	for _, l := range lumps {
		_, _ = fmt.Fprintf(stdout, "  %-4d %-8s %s\n", len(l.Data), l.Type.String(), l.Name)
	}
	_, _ = fmt.Fprintf(stdout, "Wrote %d lump(s) -> %s\n", len(lumps), out)
	return 0
}

// loadWadPalette resolves the palette for lump encoding: an explicit
// palette.lmp path wins, otherwise the built-in Quake palette.
func loadWadPalette(path string) (wad.Palette, error) {
	if path == "" {
		return wad.DefaultPalette(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return wad.Palette{}, fmt.Errorf("read palette %s: %w", path, err)
	}
	pal, err := wad.LoadPaletteBytes(data)
	if err != nil {
		return wad.Palette{}, fmt.Errorf("palette %s: %w", path, err)
	}
	return pal, nil
}

// buildWadLump decodes one image and converts it to a QPic or MipTex lump
// named after the file (cleaned, extension stripped). -type auto picks
// miptex for multiples-of-16 sizes (Quake's texture constraint) and qpic
// otherwise.
func buildWadLump(path, lumpType string, pal wad.Palette) (wad.WadLump, error) {
	img, err := image.DecodeQuakeImage(path)
	if err != nil {
		return wad.WadLump{}, err
	}
	rgba, w, h := image.RGBAFromImage(img)
	name := wad.CleanupName(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))

	if lumpType == "auto" {
		if w%16 == 0 && h%16 == 0 {
			lumpType = "miptex"
		} else {
			lumpType = "qpic"
		}
	}

	switch lumpType {
	case "qpic":
		lumpData, err := wad.WriteQPicLump(rgba, w, h, pal)
		if err != nil {
			return wad.WadLump{}, fmt.Errorf("%s: %w", path, err)
		}
		return wad.WadLump{Name: name, Type: wad.TypQPic, Data: lumpData}, nil
	case "miptex":
		lumpData, err := wad.WriteMipTexLump(name, rgba, w, h, pal)
		if err != nil {
			return wad.WadLump{}, fmt.Errorf("%s: %w", path, err)
		}
		return wad.WadLump{Name: name, Type: wad.TypMipTex, Data: lumpData}, nil
	}
	return wad.WadLump{}, fmt.Errorf("unreachable lump type %q", lumpType)
}