// Command wadgen generates a minimal Quake WAD file containing placeholder
// QPic lumps and a grayscale palette, useful for tests and tooling that need
// a valid WAD without shipping game assets.
package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
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
		fmt.Println("Usage: wadgen <output.wad>")
		return
	}
	outPath := os.Args[1]

	// Create palette (768 bytes) - grayscale
	palette := make([]byte, 768)
	for i := 0; i < 256; i++ {
		palette[i*3+0] = byte(i) // R
		palette[i*3+1] = byte(i) // G
		palette[i*3+2] = byte(i) // B
	}

	// Create dummy QPic (width, height, then pixels)
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

	// Write WAD file
	f, err := os.Create(outPath)
	if err != nil {
		panic(err)
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
