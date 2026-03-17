package image

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

// Wad2Id is the four-byte magic identifier at the start of every WAD2 file.
//
// id Software used several WAD format versions across their games:
//   - WAD (Doom) — earlier format, not used in Quake
//   - WAD2 (Quake) — stores 2D graphics lumps (console font, status bar, menus)
//   - WAD3 (Half-Life) — extends WAD2 with embedded palettes for each texture
//
// The engine reads these four ASCII bytes from the file header and rejects
// any file that does not match, preventing misidentification of unrelated data.
const (
	Wad2Id = "WAD2"
)

// LumpType specifies the type of a lump file.
//
// Each lump in a WAD2 archive has a single-byte type tag that tells the engine
// how to interpret the lump's raw data. The type determines whether the data
// has a header (like QPic's width/height prefix) or is raw pixels (like
// the conchars font bitmap stored as TypMipTex without a mip header).
//
//go:generate go tool stringer --type LumpType
type LumpType int8

// WAD2 lump type constants, matching the values defined in Quake's wad.h.
//
// These constants map directly to the original C #defines:
//   - TypNone (TYP_NONE): unused/invalid type marker
//   - TypLabel (TYP_LABEL): label-only entry, no data
//   - TypLumpy/TypPalette (TYP_LUMPY): generic lump, also used for palette data
//   - TypQTex (TYP_QTEX): Quake texture (not commonly used in gfx.wad)
//   - TypQPic (TYP_QPIC): 2D picture with a width+height header followed by pixels
//   - TypSound (TYP_SOUND): sound data (not used in gfx.wad)
//   - TypMipTex (TYP_MIPTEX): mip-mapped texture; in gfx.wad, used for conchars
//   - TypConsolePic: special type for console-related pictures
const (
	TypNone       = LumpType(0)
	TypLabel      = LumpType(1)
	TypLumpy      = LumpType(64)
	TypPalette    = LumpType(64)
	TypQTex       = LumpType(65)
	TypQPic       = LumpType(66)
	TypSound      = LumpType(67)
	TypMipTex     = LumpType(68)
	TypConsolePic = LumpType(69)
)

// WadHeader is the 12-byte header at the start of every WAD2 file.
//
// The header contains the magic identification string ("WAD2"), the total
// number of lumps in the archive, and the byte offset from the start of the
// file to the lump directory (info table). This layout allows the engine to
// seek directly to the directory without scanning the entire file.
//
// All multi-byte fields are little-endian, matching the x86 architecture
// that Quake was originally designed for.
type WadHeader struct {
	Identification [4]byte
	NumLumps       int32
	InfoTableOfs   int32
}

// LumpInfo is a 32-byte directory entry in the WAD2 info table.
//
// Each entry describes one lump: its position in the file (FilePos), its
// size on disk (DiskSize, which may differ from Size if compression were
// used — though Quake never compresses WAD lumps), its type tag, and its
// name. The Name field is a fixed 16-byte array, NUL-padded, which is
// normalized to lowercase by CleanupName during loading.
//
// The Compression field is present in the format but always zero in practice;
// Quake's WAD2 files do not use compression.
type LumpInfo struct {
	FilePos     int32
	DiskSize    int32
	Size        int32
	Type        LumpType
	Compression int8
	Pad1, Pad2  int8
	Name        [16]byte
}

// Lump represents a single parsed lump extracted from a WAD2 archive.
//
// After the WAD file is loaded, each lump is stored in memory with its
// cleaned-up name, type tag, and raw data bytes. The Data slice contains
// the lump's payload exactly as stored on disk — for a QPic lump, this
// means the 8-byte width+height header followed by palette-indexed pixels;
// for a MipTex lump (like conchars), it's raw pixel data with no header.
type Lump struct {
	Name string
	Type LumpType
	Data []byte
}

// Wad represents a loaded WAD2 archive with all lumps parsed into memory.
//
// The Lumps map is keyed by the normalized lump name (lowercase, NUL/space
// stripped). This allows O(1) lookups by name, which is how the engine
// retrieves assets — e.g., wad.Lumps["conchars"] to get the console font,
// or wad.Lumps["pause"] to get the pause screen overlay.
//
// In a typical Quake installation, gfx.wad contains roughly 30–40 lumps
// covering the console font, status bar digits, menu graphics, and other
// 2D UI assets. World textures are stored separately in BSP files, not
// in gfx.wad.
type Wad struct {
	Lumps map[string]Lump
}

// CleanupName normalizes a lump name to lowercase, strips trailing NUL bytes
// and spaces, producing a canonical key for map lookups.
//
// WAD2 lump names are stored as fixed-length 16-byte arrays padded with NUL
// bytes. The original C engine used Q_strncasecmp for case-insensitive
// comparison; the Go port instead normalizes all names to lowercase at load
// time so that standard map lookups work correctly. This function is also
// used to normalize MipTex names in BSP texture loading.
func CleanupName(name string) string {
	name = strings.ToLower(name)
	if i := strings.IndexByte(name, 0); i != -1 {
		name = name[:i]
	}
	return strings.TrimRight(name, " ")
}

// LoadWad reads and parses a complete WAD2 archive from a random-access source.
//
// The loading process mirrors the original engine's W_LoadWadFile:
//  1. Read the 12-byte header to get the magic ID, lump count, and directory offset.
//  2. Validate the magic ID is "WAD2".
//  3. Seek to the directory (info table) and read each 32-byte LumpInfo entry.
//  4. For each entry, read the lump's raw data from the file at the specified offset.
//  5. Store all lumps in a map keyed by normalized name for fast lookup.
//
// The function accepts io.ReaderAt (rather than io.Reader) because the WAD
// format requires random access: the directory is at the end of the file,
// and each lump's data is at an arbitrary offset. io.SectionReader is used
// internally to read specific byte ranges.
//
// The QPic byte-swapping comment in the loop references the original C code's
// SwapPic function, which byte-swapped width/height on big-endian platforms.
// Since Go's binary.LittleEndian handles this portably, no swap is needed.
func LoadWad(r io.ReaderAt) (*Wad, error) {
	var header WadHeader
	sr := io.NewSectionReader(r, 0, 12)
	if err := binary.Read(sr, binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	if string(header.Identification[:]) != Wad2Id {
		return nil, fmt.Errorf("not a WAD2 file: %s", string(header.Identification[:]))
	}

	lumps := make(map[string]Lump)
	sr = io.NewSectionReader(r, int64(header.InfoTableOfs), int64(header.NumLumps)*32)
	for i := 0; i < int(header.NumLumps); i++ {
		var info LumpInfo
		if err := binary.Read(sr, binary.LittleEndian, &info); err != nil {
			return nil, err
		}

		name := CleanupName(string(info.Name[:]))
		data := make([]byte, info.DiskSize)
		if _, err := r.ReadAt(data, int64(info.FilePos)); err != nil {
			return nil, err
		}

		// Handle QPic swapping if necessary
		if info.Type == TypQPic {
			// QPic has width and height as int32 at the beginning
			// wad.c calls SwapPic which does LittleLong on width and height.
			// Since we are reading from a little-endian file, they should already be correct
			// if we treat them as little-endian.
		}

		lumps[name] = Lump{
			Name: name,
			Type: info.Type,
			Data: data,
		}
	}

	return &Wad{Lumps: lumps}, nil
}

// QPic represents a Quake 2D picture: a simple width×height image stored
// as palette-indexed pixel data with an 8-byte header.
//
// The on-disk format is straightforward:
//   - Bytes 0–3: width (uint32, little-endian)
//   - Bytes 4–7: height (uint32, little-endian)
//   - Bytes 8+:  width×height palette indices (one byte per pixel, row-major)
//
// QPics are used for all 2D HUD/menu graphics in Quake: the status bar
// number sprites, menu backgrounds, loading screen images, crosshairs,
// and similar assets. They are simpler than MipTex (no mip levels, no
// name field) and are meant for screen-aligned rendering where mip-mapping
// is unnecessary.
//
// To render a QPic on a modern GPU, the engine expands each pixel byte
// through the palette (see Palette.ToRGBA) to produce an RGBA texture,
// then draws it as a screen-space quad.
type QPic struct {
	Width  uint32
	Height uint32
	Pixels []byte
}

// ParseQPic decodes a QPic from raw binary data (typically from a WAD lump
// or a standalone .lmp file loaded from a pak archive).
//
// The function validates that the data is large enough for the 8-byte header
// and the declared pixel count (width × height). The returned QPic's Pixels
// slice references the original data buffer to avoid unnecessary copying,
// so the caller should not modify the input data after parsing.
//
// A zero-sized image (width or height of 0) is rejected as invalid, since
// it would indicate corrupted data.
func ParseQPic(data []byte) (*QPic, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("qpic data too short")
	}

	width := binary.LittleEndian.Uint32(data[0:4])
	height := binary.LittleEndian.Uint32(data[4:8])
	if width == 0 || height == 0 {
		return nil, fmt.Errorf("qpic has invalid size of %dx%d", width, height)
	}

	if len(data) < 8+int(width*height) {
		return nil, fmt.Errorf("qpic data too short for %dx%d", width, height)
	}

	return &QPic{
		Width:  width,
		Height: height,
		Pixels: data[8 : 8+width*height],
	}, nil
}

// SubPic returns a new QPic containing the specified rectangular region
// of the source image. Coordinates are clamped to source bounds.
//
// This is used to extract individual glyphs or sub-images from composite
// assets. For example, the status bar number strip is a single wide QPic
// containing all digit sprites side by side; SubPic can extract a single
// digit for rendering. The returned QPic owns a freshly allocated pixel
// buffer (a copy, not a slice of the original) so it can be used
// independently.
func (p *QPic) SubPic(srcX, srcY, srcW, srcH int) *QPic {
	if srcX < 0 {
		srcX = 0
	}
	if srcY < 0 {
		srcY = 0
	}
	w := int(p.Width)
	h := int(p.Height)
	if srcX+srcW > w {
		srcW = w - srcX
	}
	if srcY+srcH > h {
		srcH = h - srcY
	}
	if srcW <= 0 || srcH <= 0 {
		return &QPic{Width: 0, Height: 0}
	}

	sub := &QPic{
		Width:  uint32(srcW),
		Height: uint32(srcH),
		Pixels: make([]byte, srcW*srcH),
	}
	for row := 0; row < srcH; row++ {
		srcOff := (srcY+row)*w + srcX
		dstOff := row * srcW
		copy(sub.Pixels[dstOff:dstOff+srcW], p.Pixels[srcOff:srcOff+srcW])
	}
	return sub
}
