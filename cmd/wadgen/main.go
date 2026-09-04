// Command wadgen generates Quake WAD files.
//
// With only an output path it emits the historical placeholder WAD (dummy
// QPic lumps + grayscale palette), useful for tests and tooling that need a
// valid WAD without shipping game assets.
//
// With image arguments it converts PNG/TGA images into QPic/MipTex lumps
// (the same conversion as `qcmod wad`). This legacy entry point is
// superseded by qcmod wad — prefer that command for new work.
package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/draw"
	"github.com/darkliquid/ironwail-go/internal/image"
)

// WAD Header
type wadHeader struct {
	Magic     [4]byte
	NumLumps  uint32
	DirOffset uint32
}

// WAD Directory Entry
type wadDirEntry struct {
	Offset uint32
	Size   uint32
	Size2  uint32 // Usually same as Size, uncompressed size
	Type   uint8
	Comp   uint8
	Pad    uint16
	Name   [16]byte
}

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

	lumps := make([]image.WadLump, 0, len(images))
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
		name := image.CleanupName(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
		switch kind {
		case "qpic":
			data, err := image.WriteQPicLump(rgba, w, h, pal)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			lumps = append(lumps, image.WadLump{Name: name, Type: image.TypQPic, Data: data})
		case "miptex":
			data, err := image.WriteMipTexLump(name, rgba, w, h, pal)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			lumps = append(lumps, image.WadLump{Name: name, Type: image.TypMipTex, Data: data})
		}
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", outPath, err)
	}
	defer func() { _ = f.Close() }()
	if err := image.WriteWad(f, lumps); err != nil {
		return err
	}
	fmt.Printf("Wrote %d lump(s) -> %s\n", len(lumps), outPath)
	return nil
}

// wadPalette resolves the encoding palette: an explicit palette.lmp path
// wins, otherwise the built-in Quake palette.
func wadPalette(palettePath string) (image.Palette, error) {
	if palettePath == "" {
		pal, err := image.LoadPaletteLmp(draw.DefaultQuakePalette())
		if err != nil {
			return image.Palette{}, err
		}
		return pal, nil
	}
	data, err := os.ReadFile(palettePath)
	if err != nil {
		return image.Palette{}, fmt.Errorf("read palette %s: %w", palettePath, err)
	}
	pal, err := image.LoadPaletteLmp(data)
	if err != nil {
		return image.Palette{}, fmt.Errorf("palette %s: %w", palettePath, err)
	}
	return pal, nil
}

// writePlaceholderWad preserves the original wadgen behaviour: a minimal
// WAD with a grayscale palette and dummy QPic lumps, for tests and tooling
// that need a valid WAD without game assets.
func writePlaceholderWad(outPath string) {
	palette := make([]byte, 768)
	for i := 0; i < 256; i++ {
		palette[i*3+0] = byte(i) // R
		palette[i*3+1] = byte(i) // G
		palette[i*3+2] = byte(i) // B
	}

	createQPic := func(width, height uint32, color byte) []byte {
		data := make([]byte, 8+width*height)
		binary.LittleEndian.PutUint32(data[0:4], width)
		binary.LittleEndian.PutUint32(data[4:8], height)
		for i := uint32(0); i < width*height; i++ {
			data[8+i] = color
		}
		return data
	}

	lumps := map[string][]byte{
		"palette.lmp":      palette,
		"gfx/qplaque.lmp":  createQPic(320, 20, 50),   // Dark gray banner
		"gfx/mainmenu.lmp": createQPic(320, 180, 100), // Mid gray menu
		"gfx/m_surfs.lmp":  createQPic(24, 20, 200),   // Light gray cursor
	}

	f, err := os.Create(outPath)
	if err != nil {
		log.Fatalf("create %s: %v", outPath, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Fatalf("close %s: %v", outPath, err)
		}
	}()

	// Write dummy header first
	hdr := wadHeader{
		Magic:    [4]byte{'W', 'A', 'D', '2'},
		NumLumps: uint32(len(lumps)),
	}
	if err := binary.Write(f, binary.LittleEndian, &hdr); err != nil {
		log.Fatalf("write header: %v", err)
	}

	// Write lump data and collect directory entries
	entries := make([]wadDirEntry, 0, len(lumps))
	for name, data := range lumps {
		offset, err := f.Seek(0, 1)
		if err != nil {
			log.Fatalf("seek: %v", err)
		}

		entry := wadDirEntry{
			Offset: uint32(offset),
			Size:   uint32(len(data)),
			Size2:  uint32(len(data)),
			Type:   69, // QPic type used in Ironwail
			Comp:   0,
		}
		if name == "palette.lmp" {
			entry.Type = 64 // Color palette
		}
		copy(entry.Name[:], name)
		entries = append(entries, entry)

		if _, err := f.Write(data); err != nil {
			log.Fatalf("write lump %s: %v", name, err)
		}
	}

	// Write directory
	dirOffset, err := f.Seek(0, 1)
	if err != nil {
		log.Fatalf("seek dir: %v", err)
	}
	for _, entry := range entries {
		if err := binary.Write(f, binary.LittleEndian, &entry); err != nil {
			log.Fatalf("write dir entry: %v", err)
		}
	}

	// Update header with correct dir offset
	if _, err := f.Seek(0, 0); err != nil {
		log.Fatalf("seek header: %v", err)
	}
	hdr.DirOffset = uint32(dirOffset)
	if err := binary.Write(f, binary.LittleEndian, &hdr); err != nil {
		log.Fatalf("rewrite header: %v", err)
	}

	fmt.Printf("Successfully created %s with %d lumps\n", outPath, len(lumps))
}