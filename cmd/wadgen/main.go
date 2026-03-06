package main

import (
	"encoding/binary"
	"fmt"
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
	defer f.Close()

	// Write dummy header first
	hdr := wadHeader{
		Magic:    [4]byte{'W', 'A', 'D', '2'},
		NumLumps: uint32(len(lumps)),
	}
	binary.Write(f, binary.LittleEndian, &hdr)

	// Write lump data and collect directory entries
	entries := make([]wadDirEntry, 0, len(lumps))
	for name, data := range lumps {
		offset, _ := f.Seek(0, 1)

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

		f.Write(data)
	}

	// Write directory
	dirOffset, _ := f.Seek(0, 1)
	for _, entry := range entries {
		binary.Write(f, binary.LittleEndian, &entry)
	}

	// Update header with correct dir offset
	f.Seek(0, 0)
	hdr.DirOffset = uint32(dirOffset)
	binary.Write(f, binary.LittleEndian, &hdr)

	fmt.Printf("Successfully created %s with %d lumps\n", outPath, len(lumps))
}
