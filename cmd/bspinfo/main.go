package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/fs"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: bspinfo <quake_dir> <gamename/maps/mapname.bsp>")
		os.Exit(1)
	}
	quakeDir := os.Args[1]
	mapPath := os.Args[2]

	fsys := fs.NewFileSystem()
	gamedir := ""
	if len(os.Args) >= 4 {
		gamedir = os.Args[3]
	}
	if err := fsys.Init(quakeDir, gamedir); err != nil {
		fmt.Fprintf(os.Stderr, "init fs: %v\n", err)
		os.Exit(1)
	}
	r, size, err := fsys.OpenFile(mapPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open %s: %v\n", mapPath, err)
		os.Exit(1)
	}
	defer r.Close()
	data := make([]byte, size)
	if _, err := r.Read(data); err != nil {
		fmt.Fprintf(os.Stderr, "read: %v\n", err)
		os.Exit(1)
	}

	tree, err := bsp.LoadTree(bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse: %v\n", err)
		os.Exit(1)
	}

	textureCount := int(binary.LittleEndian.Uint32(tree.TextureData[:4]))
	fmt.Printf("Map: %s\n", mapPath)
	fmt.Printf("Texture count: %d\n", textureCount)
	fmt.Printf("Texinfo count: %d\n", len(tree.Texinfo))
	fmt.Printf("Faces: %d\n", len(tree.Faces))
	fmt.Printf("Models: %d\n", len(tree.Models))
	fmt.Printf("TextureData lump size: %d bytes\n", len(tree.TextureData))

	fmt.Println("\nTextures:")
	for i := 0; i < textureCount; i++ {
		offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[4+i*4:])))
		if offset <= 0 {
			fmt.Printf("  [%d] <not loaded> (offset=%d)\n", i, offset)
			continue
		}
		if offset >= len(tree.TextureData) {
			fmt.Printf("  [%d] <out of range> (offset=%d)\n", i, offset)
			continue
		}
		name := string(tree.TextureData[offset : offset+16])
		for j := 0; j < 16; j++ {
			if tree.TextureData[offset+j] == 0 {
				name = string(tree.TextureData[offset : offset+j])
				break
			}
		}
		width := int32(binary.LittleEndian.Uint32(tree.TextureData[offset+16:]))
		height := int32(binary.LittleEndian.Uint32(tree.TextureData[offset+20:]))
		fmt.Printf("  [%d] %-16s %4dx%-4d\n", i, name, width, height)
	}

	// Count miptex references from texinfo
	miptexRefs := map[int32]int{}
	for _, ti := range tree.Texinfo {
		miptexRefs[ti.Miptex]++
	}
	fmt.Println("\nMiptex references from texinfo:")
	for mi, count := range miptexRefs {
		if mi >= 0 && int(mi) < textureCount {
			offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[4+int(mi)*4:])))
			if offset > 0 && offset < len(tree.TextureData) {
				name := string(tree.TextureData[offset : offset+16])
				for j := 0; j < 16; j++ {
					if tree.TextureData[offset+j] == 0 {
						name = string(tree.TextureData[offset : offset+j])
						break
					}
				}
				fmt.Printf("  miptex=%2d faces=%2d  %s\n", mi, count, name)
				continue
			}
		}
		fmt.Printf("  miptex=%2d faces=%2d  <missing>\n", mi, count)
	}

	// Check if textureCount+2 (dummy slots) would exceed 256
	if textureCount+2 > 256 {
		fmt.Printf("\nWARNING: textureCount+2=%d exceeds 256 material uniform array limit!\n", textureCount+2)
	}

	// Check for textures larger than 512 (atlas max is 2048 but check packing)
	bigCount := 0
	for i := 0; i < textureCount; i++ {
		offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[4+i*4:])))
		if offset <= 0 || offset >= len(tree.TextureData) {
			continue
		}
		width := int32(binary.LittleEndian.Uint32(tree.TextureData[offset+16:]))
		height := int32(binary.LittleEndian.Uint32(tree.TextureData[offset+20:]))
		if width > 512 || height > 512 {
			name := string(tree.TextureData[offset : offset+16])
			for j := 0; j < 16; j++ {
				if tree.TextureData[offset+j] == 0 {
					name = string(tree.TextureData[offset : offset+j])
					break
				}
			}
			fmt.Printf("  LARGE: [%d] %-16s %dx%d\n", i, name, width, height)
			bigCount++
		}
	}
	if bigCount == 0 {
		fmt.Println("\nNo textures larger than 512x512")
	}
}
