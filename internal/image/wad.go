package image

import (
	"github.com/darkliquid/ironwail-go/pkg/wad"
)

// Wad2Id is the four-byte magic identifier at the start of every WAD2 file.
const Wad2Id = wad.Wad2Id

// LumpType specifies the type of a lump file.
type LumpType = wad.LumpType

// WAD2 lump type constants, matching the values defined in Quake's wad.h.
const (
	TypNone       = wad.TypNone
	TypLabel      = wad.TypLabel
	TypLumpy      = wad.TypLumpy
	TypPalette    = wad.TypPalette
	TypQTex       = wad.TypQTex
	TypQPic       = wad.TypQPic
	TypSound      = wad.TypSound
	TypMipTex     = wad.TypMipTex
	TypConsolePic = wad.TypConsolePic
)

// WadHeader is the 12-byte header at the start of every WAD2 file.
type WadHeader = wad.Header

// LumpInfo is a 32-byte directory entry in the WAD2 info table.
type LumpInfo = wad.LumpInfo

// Lump represents a single parsed lump extracted from a WAD2 archive.
type Lump = wad.Lump

// Wad represents a loaded WAD2 archive with all lumps parsed into memory.
type Wad = wad.Wad

// CleanupName normalizes a lump name to lowercase, strips trailing NUL bytes
// and spaces, producing a canonical key for map lookups.
var CleanupName = wad.CleanupName

// LoadWad reads and parses a complete WAD2 archive from a random-access source.
var LoadWad = wad.LoadWad

// QPic represents a Quake 2D picture: a simple width×height image stored
// as palette-indexed pixel data with an 8-byte header.
type QPic = wad.QPic

// ParseQPic decodes a QPic from raw binary data.
var ParseQPic = wad.ParseQPic
