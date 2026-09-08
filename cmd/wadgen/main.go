// Command wadgen generates Quake WAD files.
//
// Deprecated: wadgen is superseded by `qcmod wad` — prefer that command for new work.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/pkg/wad"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: wadgen <output.wad> [image.png|image.tga ...] [-type qpic|miptex] [-palette palette.lmp]")
		os.Exit(2)
	}

	outPath := os.Args[1]
	images, lumpType, palettePath, err := parseArgs(os.Args[2:])
	if err != nil {
		log.Fatalf("wadgen: %v", err)
	}

	if len(images) == 0 {
		writePlaceholderWad(outPath)
		return
	}

	fmt.Fprintf(os.Stderr, "wadgen: note: image conversion is superseded by `qcmod wad`\n")
	if err := writeImageWad(outPath, images, lumpType, palettePath); err != nil {
		log.Fatalf("wadgen: %v", err)
	}
}

// parseArgs scans interleaved flags and positional image paths.
func parseArgs(args []string) (images []string, lumpType, palettePath string, err error) {
	lumpType = "auto"
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-type" || strings.HasPrefix(a, "-type="):
			v := strings.TrimPrefix(a, "-type=")
			if a == "-type" {
				if i+1 >= len(args) {
					return nil, "", "", fmt.Errorf("-type requires a value")
				}
				i++
				v = args[i]
			}
			switch v {
			case "auto", "qpic", "miptex":
				lumpType = v
			default:
				return nil, "", "", fmt.Errorf("unknown -type %q (auto|qpic|miptex)", v)
			}
		case a == "-palette" || a == "-pal" || strings.HasPrefix(a, "-palette="):
			v := strings.TrimPrefix(a, "-palette=")
			if a == "-palette" || a == "-pal" {
				if i+1 >= len(args) {
					return nil, "", "", fmt.Errorf("%s requires a value", a)
				}
				i++
				v = args[i]
			}
			palettePath = v
		case strings.HasPrefix(a, "-"):
			return nil, "", "", fmt.Errorf("unknown flag %q", a)
		default:
			images = append(images, a)
		}
	}
	return images, lumpType, palettePath, nil
}

// writeImageWad converts each image into a lump and writes the WAD.
func writeImageWad(outPath string, images []string, lumpType, palettePath string) error {
	pal, err := wadPalette(palettePath)
	if err != nil {
		return err
	}

	lumps := make([]wad.WadLump, 0, len(images))
	for _, path := range images {
		img, err := image.DecodeQuakeImage(path)
		if err != nil {
			return err
		}
		rgba, w, h := image.RGBAFromImage(img)
		kind := lumpType
		if kind == "auto" {
			if w%16 == 0 && h%16 == 0 {
				kind = "miptex"
			} else {
				kind = "qpic"
			}
		}
		name := wad.CleanupName(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
		switch kind {
		case "qpic":
			data, err := wad.WriteQPicLump(rgba, w, h, pal)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			lumps = append(lumps, wad.WadLump{Name: name, Type: wad.TypQPic, Data: data})
		case "miptex":
			data, err := wad.WriteMipTexLump(name, rgba, w, h, pal)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			lumps = append(lumps, wad.WadLump{Name: name, Type: wad.TypMipTex, Data: data})
		}
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", outPath, err)
	}
	defer func() { _ = f.Close() }()
	if err := wad.WriteWad(f, lumps); err != nil {
		return err
	}
	fmt.Printf("Wrote %d lump(s) -> %s\n", len(lumps), outPath)
	return nil
}

// wadPalette resolves the encoding palette: an explicit palette.lmp path
// wins, otherwise the built-in Quake palette.
func wadPalette(palettePath string) (wad.Palette, error) {
	if palettePath == "" {
		return wad.DefaultPalette(), nil
	}
	data, err := os.ReadFile(palettePath)
	if err != nil {
		return wad.Palette{}, fmt.Errorf("read palette %s: %w", palettePath, err)
	}
	return wad.LoadPaletteBytes(data)
}

// writePlaceholderWad emits a minimal WAD with placeholder lumps for tests and tooling.
func writePlaceholderWad(outPath string) {
	f, err := os.Create(outPath)
	if err != nil {
		log.Fatalf("create %s: %v", outPath, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Fatalf("close %s: %v", outPath, err)
		}
	}()

	if err := wad.WritePlaceholderWad(f); err != nil {
		log.Fatalf("write placeholder wad %s: %v", outPath, err)
	}
	fmt.Printf("Successfully created %s with placeholder lumps\n", outPath)
}
