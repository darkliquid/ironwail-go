// Package bsp provides reading and parsing of Quake BSP files.
// BSP (Binary Space Partitioning) files contain the geometry, textures,
// lighting, and visibility data for Quake maps.
//
// The BSP file format is a lump-based binary format. The file starts with
// a header containing a version number and 15 lump descriptors, each
// providing an offset and length into the file for a specific data type.
//
// Supported BSP Versions:
//   - BSPVersion (29): Original Quake format
//   - BSP2Version_2PSB: RMQ format for large maps
//   - BSP2Version_BSP2: BSP2 format with 32-bit indices
//   - BSPVersion_Quake64: Quake 64 format
//
// File Structure:
//
//	+------------------+
//	| Header (16 ints) |
//	+------------------+
//	| Entities Lump    |
//	+------------------+
//	| Planes Lump      |
//	+------------------+
//	| ... (13 more)    |
//	+------------------+
//
// The package provides both on-disk structures (prefixed with D) for
// reading from files, and in-memory structures for runtime use.
package bsp

import (
	"encoding/binary"
	"io"
)

// BSP version constants
const (
	BSPVersion         = 29                                                  // Original Quake BSP version
	BSP2Version_2PSB   = ('B' << 24) | ('S' << 16) | ('P' << 8) | '2'        // RMQ format
	BSP2Version_BSP2   = ('B' << 0) | ('S' << 8) | ('P' << 16) | ('2' << 24) // BSP2 format
	BSPVersion_Quake64 = ('Q' << 24) | ('6' << 16) | ('4' << 8) | ' '        // Quake 64
)

// Lump type indices
const (
	LumpEntities     = 0
	LumpPlanes       = 1
	LumpTextures     = 2
	LumpVertexes     = 3
	LumpVisibility   = 4
	LumpNodes        = 5
	LumpTexinfo      = 6
	LumpFaces        = 7
	LumpLighting     = 8
	LumpClipnodes    = 9
	LumpLeafs        = 10
	LumpMarksurfaces = 11
	LumpEdges        = 12
	LumpSurfedges    = 13
	LumpModels       = 14
	HeaderLumps      = 15
)

// Maximum map limits
const (
	MaxMapHulls        = 4
	MaxMapModels       = 256
	MaxMapBrushes      = 4096
	MaxMapEntities     = 1024
	MaxMapEntstring    = 65536
	MaxMapPlanes       = 32767
	MaxMapNodes        = 32767
	MaxMapClipnodes    = 32767
	MaxMapVerts        = 65535
	MaxMapFaces        = 65535
	MaxMapMarksurfaces = 65535
	MaxMapTexinfo      = 4096
	MaxMapEdges        = 256000
	MaxMapSurfedges    = 512000
	MaxMapTextures     = 512
	MaxMapMiptex       = 0x200000
	MaxMapLighting     = 0x100000
	MaxMapVisibility   = 0x100000
	MaxMapPortals      = 65536
)

// Entity key/value pair limits
const (
	MaxKey   = 32
	MaxValue = 1024
)

// Mipmap levels
const (
	MipLevels = 4
)

// Content types for leafs
const (
	ContentsEmpty  = -1
	ContentsSolid  = -2
	ContentsWater  = -3
	ContentsSlime  = -4
	ContentsLava   = -5
	ContentsSky    = -6
	ContentsOrigin = -7 // Removed at CSG time
	ContentsClip   = -8 // Changed to ContentsSolid

	ContentsCurrent0    = -9
	ContentsCurrent90   = -10
	ContentsCurrent180  = -11
	ContentsCurrent270  = -12
	ContentsCurrentUp   = -13
	ContentsCurrentDown = -14
)

// Plane types
const (
	PlaneX    = 0 // Axial planes
	PlaneY    = 1
	PlaneZ    = 2
	PlaneAnyX = 3 // Non-axial planes snapped to nearest
	PlaneAnyY = 4
	PlaneAnyZ = 5
)

// Ambient sound indices
const (
	AmbientWater = iota
	AmbientSky
	AmbientSlime
	AmbientLava
	NumAmbients
)

// MaxLightmaps is the maximum number of lightmap styles per face
const MaxLightmaps = 4

// Texinfo flags
const (
	TexSpecial = 1 // Sky or slime, no lightmap or 256 subdivision
	TexMissing = 2 // This texinfo does not have a texture
)

// Lump represents a section of data in the BSP file.
// Each lump has an offset and length that points to the data.
type Lump struct {
	FileOffset int32
	FileLength int32
}

// DHeader is the BSP file header.
// It contains the version number and 15 lump descriptors.
type DHeader struct {
	Version int32
	Lumps   [HeaderLumps]Lump
}

// DModel represents a sub-model in the BSP file.
// The first model (index 0) is the world, and subsequent models
// are brush entities like doors and platforms.
type DModel struct {
	BoundsMin [3]float32
	BoundsMax [3]float32
	Origin    [3]float32
	HeadNode  [MaxMapHulls]int32
	VisLeafs  int32 // Not including solid leaf 0
	FirstFace int32
	NumFaces  int32
}

// DMiptexLump is the header for the textures lump.
// It contains the number of textures and offsets to each miptex.
type DMiptexLump struct {
	NumMiptex int32
	DataOfs   [MaxMapTextures]int32
}

// Miptex represents a miptexture (mipmapped texture).
// It contains the texture name, dimensions, and offsets to
// the four mipmap levels.
type Miptex struct {
	Name    [16]byte
	Width   uint32
	Height  uint32
	Offsets [MipLevels]uint32
}

// Miptex64 is the Quake 64 variant of miptex with an additional shift field.
type Miptex64 struct {
	Name    [16]byte
	Width   uint32
	Height  uint32
	Shift   uint32
	Offsets [MipLevels]uint32
}

// DVertex represents a vertex in the BSP.
type DVertex struct {
	Point [3]float32
}

// DPlane represents a plane in the BSP tree.
// Planes are used for spatial partitioning.
type DPlane struct {
	Normal [3]float32
	Dist   float32
	Type   int32 // PlaneX - PlaneAnyZ
}

// DSNode is a standard BSP node (16-bit indices).
// Used in the original Quake BSP format.
type DSNode struct {
	PlaneNum  int32
	Children  [2]int16 // Negative numbers are -(leafs+1)
	BoundsMin [3]int16 // For sphere culling
	BoundsMax [3]int16
	FirstFace uint16
	NumFaces  uint16
}

// DL1Node is a BSP2 level 1 node (32-bit indices, 16-bit bounds).
type DL1Node struct {
	PlaneNum  int32
	Children  [2]int32
	BoundsMin [3]int16
	BoundsMax [3]int16
	FirstFace uint32
	NumFaces  uint32
}

// DL2Node is a BSP2 level 2 node (32-bit indices, float bounds).
type DL2Node struct {
	PlaneNum  int32
	Children  [2]int32
	BoundsMin [3]float32
	BoundsMax [3]float32
	FirstFace uint32
	NumFaces  uint32
}

// DSClipNode is a standard clip node (16-bit children).
type DSClipNode struct {
	PlaneNum int32
	Children [2]int16 // Negative numbers are contents
}

// DLClipNode is a BSP2 clip node (32-bit children).
type DLClipNode struct {
	PlaneNum int32
	Children [2]int32
}

// Texinfo contains texture mapping information.
type Texinfo struct {
	Vecs   [2][4]float32 // [s/t][xyz offset] for texture mapping
	Miptex int32
	Flags  int32
}

// DSEdge is a standard edge (16-bit vertex indices).
type DSEdge struct {
	V [2]uint16 // Vertex numbers
}

// DLEdge is a BSP2 edge (32-bit vertex indices).
type DLEdge struct {
	V [2]uint32
}

// DSFace is a standard BSP face (16-bit edge count).
type DSFace struct {
	PlaneNum  int16
	Side      int16
	FirstEdge int32
	NumEdges  int16
	Texinfo   int16
	Styles    [MaxLightmaps]uint8
	LightOfs  int32
}

// DLFace is a BSP2 face (32-bit edge count).
type DLFace struct {
	PlaneNum  int32
	Side      int32
	FirstEdge int32
	NumEdges  int32
	Texinfo   int32
	Styles    [MaxLightmaps]uint8
	LightOfs  int32
}

// DSLeaf is a standard BSP leaf (16-bit surface indices).
// Leaf 0 is the generic CONTENTS_SOLID leaf used for all solid areas.
type DSLeaf struct {
	Contents         int32
	VisOfs           int32 // -1 = no visibility info
	BoundsMin        [3]int16
	BoundsMax        [3]int16
	FirstMarkSurface uint16
	NumMarkSurfaces  uint16
	AmbientLevel     [NumAmbients]uint8
}

// DL1Leaf is a BSP2 level 1 leaf (32-bit surface indices, 16-bit bounds).
type DL1Leaf struct {
	Contents         int32
	VisOfs           int32
	BoundsMin        [3]int16
	BoundsMax        [3]int16
	FirstMarkSurface uint32
	NumMarkSurfaces  uint32
	AmbientLevel     [NumAmbients]uint8
}

// DL2Leaf is a BSP2 level 2 leaf (32-bit indices, float bounds).
type DL2Leaf struct {
	Contents         int32
	VisOfs           int32
	BoundsMin        [3]float32
	BoundsMax        [3]float32
	FirstMarkSurface uint32
	NumMarkSurfaces  uint32
	AmbientLevel     [NumAmbients]uint8
}

// File represents a parsed BSP file.
type File struct {
	Header  DHeader
	Version int32

	// Raw lump data (loaded but not parsed)
	Entities   []byte
	Planes     []DPlane
	Vertexes   []DVertex
	Visibility []byte
	Texinfo    []Texinfo
	Lighting   []byte

	// Version-dependent structures (one will be populated based on BSP version)
	Nodes        interface{} // []DSNode, []DL1Node, or []DL2Node
	Clipnodes    interface{} // []DSClipNode or []DLClipNode
	Leafs        interface{} // []DSLeaf, []DL1Leaf, or []DL2Leaf
	Faces        interface{} // []DSFace or []DLFace
	Edges        interface{} // []DSEdge or []DLEdge
	MarkSurfaces interface{} // []uint16 or []uint32
	Surfedges    []int32
	Models       []DModel

	// Texture data
	NumTextures int32
	TextureData []byte

	// Metadata
	IsBSP2    bool
	IsQuake64 bool
}

// Reader reads BSP file data from an io.ReadSeeker.
type Reader struct {
	r         io.ReadSeeker
	byteOrder binary.ByteOrder
}

// NewReader creates a new BSP reader.
func NewReader(r io.ReadSeeker) *Reader {
	return &Reader{
		r:         r,
		byteOrder: binary.LittleEndian, // Quake BSP files are little-endian
	}
}

// ReadHeader reads and validates the BSP header.
func (r *Reader) ReadHeader() (*DHeader, error) {
	var header DHeader
	if err := binary.Read(r.r, r.byteOrder, &header.Version); err != nil {
		return nil, err
	}

	for i := 0; i < HeaderLumps; i++ {
		if err := binary.Read(r.r, r.byteOrder, &header.Lumps[i]); err != nil {
			return nil, err
		}
	}

	return &header, nil
}

// ReadLump reads a lump's data from the file.
func (r *Reader) ReadLump(lump *Lump) ([]byte, error) {
	if lump.FileLength == 0 {
		return nil, nil
	}

	data := make([]byte, lump.FileLength)
	if _, err := r.r.Seek(int64(lump.FileOffset), io.SeekStart); err != nil {
		return nil, err
	}
	if _, err := io.ReadFull(r.r, data); err != nil {
		return nil, err
	}
	return data, nil
}

// IsBSP2 returns true if the version indicates a BSP2 format.
func IsBSP2(version int32) bool {
	return version == BSP2Version_2PSB || version == BSP2Version_BSP2
}

// IsQuake64 returns true if the version indicates Quake 64 format.
func IsQuake64(version int32) bool {
	return version == BSPVersion_Quake64
}

// IsValidVersion returns true if the version is a known BSP format.
func IsValidVersion(version int32) bool {
	return version == BSPVersion ||
		IsBSP2(version) ||
		IsQuake64(version)
}
