// Package bsp provides reading, parsing, writing, and lump-patching of Quake BSP files.
// BSP (Binary Space Partitioning) files contain geometry, textures, lighting,
// visibility, and entity definitions for Quake maps.
//
// Supported BSP Versions:
//   - BSPVersion (29): Original Quake format
//   - BSP2Version_2PSB: RMQ format for large maps
//   - BSP2Version_BSP2: BSP2 format with 32-bit indices
//   - BSPVersion_Quake64: Quake 64 format
package bsp

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/darkliquid/ironwail-go/pkg/types"
)

// BSP version constants
const (
	BSPVersion         = 29                                                   // Original Quake BSP version
	BSP2Version_2PSB   = ('B' << 24) | ('S' << 16) | ('P' << 8) | '2'         // RMQ format
	BSP2Version_BSP2   = ('B' << 0) | ('S' << 8) | ('P' << 16) | ('2' << 24)  // BSP2 format
	BSPVersion_Quake64 = ('Q' << 24) | ('6' << 16) | ('4' << 8) | ' '         // Quake 64
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
type Lump struct {
	FileOffset int32
	FileLength int32
}

// DHeader is the BSP file header containing version and lump descriptors.
type DHeader struct {
	Version int32
	Lumps   [HeaderLumps]Lump
}

// DModel represents a sub-model in the BSP file.
type DModel struct {
	BoundsMin types.Vec3
	BoundsMax types.Vec3
	Origin    types.Vec3
	HeadNode  [MaxMapHulls]int32
	VisLeafs  int32 // Not including solid leaf 0
	FirstFace int32
	NumFaces  int32
}

// DMiptexLump is the header for the textures lump.
type DMiptexLump struct {
	NumMiptex int32
	DataOfs   [MaxMapTextures]int32
}

// Miptex represents a miptexture header in the textures lump.
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
	Point types.Vec3
}

// DPlane represents a plane in the BSP tree.
type DPlane struct {
	Normal types.Vec3
	Dist   float32
	Type   int32 // PlaneX - PlaneAnyZ
}

// DSNode is a standard BSP node (16-bit indices).
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
	BoundsMin types.Vec3
	BoundsMax types.Vec3
	FirstFace uint32
	NumFaces  uint32
}

// DSClipNode is a standard clip node.
type DSClipNode struct {
	PlaneNum int32
	Children [2]int32 // Interpreted as unsigned 16-bit integers on disk
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
	V [2]uint16
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
	BoundsMin        types.Vec3
	BoundsMax        types.Vec3
	FirstMarkSurface uint32
	NumMarkSurfaces  uint32
	AmbientLevel     [NumAmbients]uint8
}

// File represents a parsed low-level BSP file with on-disk format records.
type File struct {
	Header  DHeader
	Version int32

	Entities   []byte
	Planes     []DPlane
	Vertexes   []DVertex
	Visibility []byte
	Texinfo    []Texinfo
	Lighting   []byte

	Nodes        any // []DSNode, []DL1Node, or []DL2Node
	Clipnodes    any // []DSClipNode or []DLClipNode
	Leafs        any // []DSLeaf, []DL1Leaf, or []DL2Leaf
	Faces        any // []DSFace or []DLFace
	Edges        any // []DSEdge or []DLEdge
	MarkSurfaces any // []uint16 or []uint32
	Surfedges    []int32
	Models       []DModel

	NumTextures int32
	TextureData []byte

	IsBSP2    bool
	IsQuake64 bool
}

// Reader reads BSP data from an io.ReadSeeker.
type Reader struct {
	r         io.ReadSeeker
	byteOrder binary.ByteOrder
}

// NewReader creates a new BSP reader.
func NewReader(r io.ReadSeeker) *Reader {
	return &Reader{
		r:         r,
		byteOrder: binary.LittleEndian,
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

// ReadLump reads a lump's raw bytes from the file.
func (r *Reader) ReadLump(lump *Lump) ([]byte, error) {
	if lump.FileLength == 0 {
		return nil, nil
	}
	if lump.FileLength < 0 {
		return nil, fmt.Errorf("invalid lump length: %d", lump.FileLength)
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

// IsValidVersion returns true if the version is a recognized BSP format.
func IsValidVersion(version int32) bool {
	return version == BSPVersion ||
		IsBSP2(version) ||
		IsQuake64(version)
}

// Load reads and parses a complete BSP file into a *File structure.
func Load(r io.ReadSeeker) (*File, error) {
	reader := NewReader(r)
	header, err := reader.ReadHeader()
	if err != nil {
		return nil, fmt.Errorf("read BSP header: %w", err)
	}
	if !IsValidVersion(header.Version) {
		return nil, fmt.Errorf("unsupported BSP version: %d", header.Version)
	}

	file := &File{
		Header:    *header,
		Version:   header.Version,
		IsBSP2:    IsBSP2(header.Version),
		IsQuake64: IsQuake64(header.Version),
	}

	// 0: Entities
	if data, err := reader.ReadLump(&file.Header.Lumps[LumpEntities]); err != nil {
		return nil, fmt.Errorf("load entities: %w", err)
	} else {
		file.Entities = data
	}

	// 1: Planes
	if data, err := reader.ReadLump(&file.Header.Lumps[LumpPlanes]); err != nil {
		return nil, fmt.Errorf("load planes: %w", err)
	} else if len(data) > 0 {
		const recSize = 20
		if len(data)%recSize != 0 {
			return nil, fmt.Errorf("planes lump funny size %d", len(data))
		}
		count := len(data) / recSize
		file.Planes = make([]DPlane, count)
		for i := 0; i < count; i++ {
			off := i * recSize
			file.Planes[i] = DPlane{
				Normal: types.Vec3{
					X: math.Float32frombits(binary.LittleEndian.Uint32(data[off:])),
					Y: math.Float32frombits(binary.LittleEndian.Uint32(data[off+4:])),
					Z: math.Float32frombits(binary.LittleEndian.Uint32(data[off+8:])),
				},
				Dist: math.Float32frombits(binary.LittleEndian.Uint32(data[off+12:])),
				Type: int32(binary.LittleEndian.Uint32(data[off+16:])),
			}
		}
	}

	// 2: Textures
	if data, err := reader.ReadLump(&file.Header.Lumps[LumpTextures]); err != nil {
		return nil, fmt.Errorf("load textures: %w", err)
	} else if len(data) >= 4 {
		file.NumTextures = int32(binary.LittleEndian.Uint32(data[:4]))
		file.TextureData = data
	}

	// 3: Vertexes
	if data, err := reader.ReadLump(&file.Header.Lumps[LumpVertexes]); err != nil {
		return nil, fmt.Errorf("load vertexes: %w", err)
	} else if len(data) > 0 {
		const recSize = 12
		if len(data)%recSize != 0 {
			return nil, fmt.Errorf("vertexes lump funny size %d", len(data))
		}
		count := len(data) / recSize
		file.Vertexes = make([]DVertex, count)
		for i := 0; i < count; i++ {
			off := i * recSize
			file.Vertexes[i] = DVertex{
				Point: types.Vec3{
					X: math.Float32frombits(binary.LittleEndian.Uint32(data[off:])),
					Y: math.Float32frombits(binary.LittleEndian.Uint32(data[off+4:])),
					Z: math.Float32frombits(binary.LittleEndian.Uint32(data[off+8:])),
				},
			}
		}
	}

	// 4: Visibility
	if data, err := reader.ReadLump(&file.Header.Lumps[LumpVisibility]); err != nil {
		return nil, fmt.Errorf("load visibility: %w", err)
	} else {
		file.Visibility = data
	}

	// 5: Nodes
	if data, err := reader.ReadLump(&file.Header.Lumps[LumpNodes]); err != nil {
		return nil, fmt.Errorf("load nodes: %w", err)
	} else if len(data) > 0 {
		switch file.Version {
		case BSP2Version_BSP2:
			const recSize = 44
			if len(data)%recSize != 0 {
				return nil, fmt.Errorf("nodes lump funny size %d", len(data))
			}
			count := len(data) / recSize
			nodes := make([]DL2Node, count)
			for i := 0; i < count; i++ {
				off := i * recSize
				nodes[i] = DL2Node{
					PlaneNum: int32(binary.LittleEndian.Uint32(data[off:])),
					Children: [2]int32{
						int32(binary.LittleEndian.Uint32(data[off+4:])),
						int32(binary.LittleEndian.Uint32(data[off+8:])),
					},
					BoundsMin: types.Vec3{
						X: math.Float32frombits(binary.LittleEndian.Uint32(data[off+12:])),
						Y: math.Float32frombits(binary.LittleEndian.Uint32(data[off+16:])),
						Z: math.Float32frombits(binary.LittleEndian.Uint32(data[off+20:])),
					},
					BoundsMax: types.Vec3{
						X: math.Float32frombits(binary.LittleEndian.Uint32(data[off+24:])),
						Y: math.Float32frombits(binary.LittleEndian.Uint32(data[off+28:])),
						Z: math.Float32frombits(binary.LittleEndian.Uint32(data[off+32:])),
					},
					FirstFace: binary.LittleEndian.Uint32(data[off+36:]),
					NumFaces:  binary.LittleEndian.Uint32(data[off+40:]),
				}
			}
			file.Nodes = nodes
		case BSP2Version_2PSB:
			const recSize = 32
			if len(data)%recSize != 0 {
				return nil, fmt.Errorf("nodes lump funny size %d", len(data))
			}
			count := len(data) / recSize
			nodes := make([]DL1Node, count)
			for i := 0; i < count; i++ {
				off := i * recSize
				nodes[i] = DL1Node{
					PlaneNum: int32(binary.LittleEndian.Uint32(data[off:])),
					Children: [2]int32{
						int32(binary.LittleEndian.Uint32(data[off+4:])),
						int32(binary.LittleEndian.Uint32(data[off+8:])),
					},
					BoundsMin: [3]int16{
						int16(binary.LittleEndian.Uint16(data[off+12:])),
						int16(binary.LittleEndian.Uint16(data[off+14:])),
						int16(binary.LittleEndian.Uint16(data[off+16:])),
					},
					BoundsMax: [3]int16{
						int16(binary.LittleEndian.Uint16(data[off+18:])),
						int16(binary.LittleEndian.Uint16(data[off+20:])),
						int16(binary.LittleEndian.Uint16(data[off+22:])),
					},
					FirstFace: binary.LittleEndian.Uint32(data[off+24:]),
					NumFaces:  binary.LittleEndian.Uint32(data[off+28:]),
				}
			}
			file.Nodes = nodes
		default:
			const recSize = 24
			if len(data)%recSize != 0 {
				return nil, fmt.Errorf("nodes lump funny size %d", len(data))
			}
			count := len(data) / recSize
			nodes := make([]DSNode, count)
			for i := 0; i < count; i++ {
				off := i * recSize
				nodes[i] = DSNode{
					PlaneNum: int32(binary.LittleEndian.Uint32(data[off:])),
					Children: [2]int16{
						int16(binary.LittleEndian.Uint16(data[off+4:])),
						int16(binary.LittleEndian.Uint16(data[off+6:])),
					},
					BoundsMin: [3]int16{
						int16(binary.LittleEndian.Uint16(data[off+8:])),
						int16(binary.LittleEndian.Uint16(data[off+10:])),
						int16(binary.LittleEndian.Uint16(data[off+12:])),
					},
					BoundsMax: [3]int16{
						int16(binary.LittleEndian.Uint16(data[off+14:])),
						int16(binary.LittleEndian.Uint16(data[off+16:])),
						int16(binary.LittleEndian.Uint16(data[off+18:])),
					},
					FirstFace: uint16(binary.LittleEndian.Uint16(data[off+20:])),
					NumFaces:  uint16(binary.LittleEndian.Uint16(data[off+22:])),
				}
			}
			file.Nodes = nodes
		}
	}

	// 6: Texinfo
	if data, err := reader.ReadLump(&file.Header.Lumps[LumpTexinfo]); err != nil {
		return nil, fmt.Errorf("load texinfo: %w", err)
	} else if len(data) > 0 {
		const recSize = 40
		if len(data)%recSize != 0 {
			return nil, fmt.Errorf("texinfo lump funny size %d", len(data))
		}
		count := len(data) / recSize
		file.Texinfo = make([]Texinfo, count)
		for i := 0; i < count; i++ {
			off := i * recSize
			var ti Texinfo
			for s := 0; s < 2; s++ {
				for c := 0; c < 4; c++ {
					vOff := off + (s*4+c)*4
					ti.Vecs[s][c] = math.Float32frombits(binary.LittleEndian.Uint32(data[vOff:]))
				}
			}
			ti.Miptex = int32(binary.LittleEndian.Uint32(data[off+32:]))
			ti.Flags = int32(binary.LittleEndian.Uint32(data[off+36:]))
			file.Texinfo[i] = ti
		}
	}

	// 7: Faces
	if data, err := reader.ReadLump(&file.Header.Lumps[LumpFaces]); err != nil {
		return nil, fmt.Errorf("load faces: %w", err)
	} else if len(data) > 0 {
		if file.IsBSP2 {
			const recSize = 28
			if len(data)%recSize != 0 {
				return nil, fmt.Errorf("faces lump funny size %d", len(data))
			}
			count := len(data) / recSize
			faces := make([]DLFace, count)
			for i := 0; i < count; i++ {
				off := i * recSize
				faces[i] = DLFace{
					PlaneNum:  int32(binary.LittleEndian.Uint32(data[off:])),
					Side:      int32(binary.LittleEndian.Uint32(data[off+4:])),
					FirstEdge: int32(binary.LittleEndian.Uint32(data[off+8:])),
					NumEdges:  int32(binary.LittleEndian.Uint32(data[off+12:])),
					Texinfo:   int32(binary.LittleEndian.Uint32(data[off+16:])),
					Styles:    [MaxLightmaps]uint8{data[off+20], data[off+21], data[off+22], data[off+23]},
					LightOfs:  int32(binary.LittleEndian.Uint32(data[off+24:])),
				}
			}
			file.Faces = faces
		} else {
			const recSize = 20
			if len(data)%recSize != 0 {
				return nil, fmt.Errorf("faces lump funny size %d", len(data))
			}
			count := len(data) / recSize
			faces := make([]DSFace, count)
			for i := 0; i < count; i++ {
				off := i * recSize
				faces[i] = DSFace{
					PlaneNum:  int16(binary.LittleEndian.Uint16(data[off:])),
					Side:      int16(binary.LittleEndian.Uint16(data[off+2:])),
					FirstEdge: int32(binary.LittleEndian.Uint32(data[off+4:])),
					NumEdges:  int16(binary.LittleEndian.Uint16(data[off+8:])),
					Texinfo:   int16(binary.LittleEndian.Uint16(data[off+10:])),
					Styles:    [MaxLightmaps]uint8{data[off+12], data[off+13], data[off+14], data[off+15]},
					LightOfs:  int32(binary.LittleEndian.Uint32(data[off+16:])),
				}
			}
			file.Faces = faces
		}
	}

	// 8: Lighting
	if data, err := reader.ReadLump(&file.Header.Lumps[LumpLighting]); err != nil {
		return nil, fmt.Errorf("load lighting: %w", err)
	} else {
		file.Lighting = data
	}

	// 9: Clipnodes
	if data, err := reader.ReadLump(&file.Header.Lumps[LumpClipnodes]); err != nil {
		return nil, fmt.Errorf("load clipnodes: %w", err)
	} else if len(data) > 0 {
		if file.IsBSP2 {
			const recSize = 12
			if len(data)%recSize != 0 {
				return nil, fmt.Errorf("clipnodes lump funny size %d", len(data))
			}
			count := len(data) / recSize
			cns := make([]DLClipNode, count)
			for i := 0; i < count; i++ {
				off := i * recSize
				cns[i] = DLClipNode{
					PlaneNum: int32(binary.LittleEndian.Uint32(data[off:])),
					Children: [2]int32{
						int32(binary.LittleEndian.Uint32(data[off+4:])),
						int32(binary.LittleEndian.Uint32(data[off+8:])),
					},
				}
			}
			file.Clipnodes = cns
		} else {
			const recSize = 8
			if len(data)%recSize != 0 {
				return nil, fmt.Errorf("clipnodes lump funny size %d", len(data))
			}
			count := len(data) / recSize
			cns := make([]DSClipNode, count)
			for i := 0; i < count; i++ {
				off := i * recSize
				c0 := int32(int16(binary.LittleEndian.Uint16(data[off+4:])))
				c1 := int32(int16(binary.LittleEndian.Uint16(data[off+6:])))
				cns[i] = DSClipNode{
					PlaneNum: int32(binary.LittleEndian.Uint32(data[off:])),
					Children: [2]int32{c0, c1},
				}
			}
			file.Clipnodes = cns
		}
	}

	// 10: Leafs
	if data, err := reader.ReadLump(&file.Header.Lumps[LumpLeafs]); err != nil {
		return nil, fmt.Errorf("load leafs: %w", err)
	} else if len(data) > 0 {
		switch file.Version {
		case BSP2Version_BSP2:
			const recSize = 44
			if len(data)%recSize != 0 {
				return nil, fmt.Errorf("leafs lump funny size %d", len(data))
			}
			count := len(data) / recSize
			leafs := make([]DL2Leaf, count)
			for i := 0; i < count; i++ {
				off := i * recSize
				leafs[i] = DL2Leaf{
					Contents: int32(binary.LittleEndian.Uint32(data[off:])),
					VisOfs:   int32(binary.LittleEndian.Uint32(data[off+4:])),
					BoundsMin: types.Vec3{
						X: math.Float32frombits(binary.LittleEndian.Uint32(data[off+8:])),
						Y: math.Float32frombits(binary.LittleEndian.Uint32(data[off+12:])),
						Z: math.Float32frombits(binary.LittleEndian.Uint32(data[off+16:])),
					},
					BoundsMax: types.Vec3{
						X: math.Float32frombits(binary.LittleEndian.Uint32(data[off+20:])),
						Y: math.Float32frombits(binary.LittleEndian.Uint32(data[off+24:])),
						Z: math.Float32frombits(binary.LittleEndian.Uint32(data[off+28:])),
					},
					FirstMarkSurface: binary.LittleEndian.Uint32(data[off+32:]),
					NumMarkSurfaces:  binary.LittleEndian.Uint32(data[off+36:]),
					AmbientLevel:     [NumAmbients]uint8{data[off+40], data[off+41], data[off+42], data[off+43]},
				}
			}
			file.Leafs = leafs
		case BSP2Version_2PSB:
			const recSize = 32
			if len(data)%recSize != 0 {
				return nil, fmt.Errorf("leafs lump funny size %d", len(data))
			}
			count := len(data) / recSize
			leafs := make([]DL1Leaf, count)
			for i := 0; i < count; i++ {
				off := i * recSize
				leafs[i] = DL1Leaf{
					Contents: int32(binary.LittleEndian.Uint32(data[off:])),
					VisOfs:   int32(binary.LittleEndian.Uint32(data[off+4:])),
					BoundsMin: [3]int16{
						int16(binary.LittleEndian.Uint16(data[off+8:])),
						int16(binary.LittleEndian.Uint16(data[off+10:])),
						int16(binary.LittleEndian.Uint16(data[off+12:])),
					},
					BoundsMax: [3]int16{
						int16(binary.LittleEndian.Uint16(data[off+14:])),
						int16(binary.LittleEndian.Uint16(data[off+16:])),
						int16(binary.LittleEndian.Uint16(data[off+18:])),
					},
					FirstMarkSurface: binary.LittleEndian.Uint32(data[off+20:]),
					NumMarkSurfaces:  binary.LittleEndian.Uint32(data[off+24:]),
					AmbientLevel:     [NumAmbients]uint8{data[off+28], data[off+29], data[off+30], data[off+31]},
				}
			}
			file.Leafs = leafs
		default:
			const recSize = 28
			if len(data)%recSize != 0 {
				return nil, fmt.Errorf("leafs lump funny size %d", len(data))
			}
			count := len(data) / recSize
			leafs := make([]DSLeaf, count)
			for i := 0; i < count; i++ {
				off := i * recSize
				leafs[i] = DSLeaf{
					Contents: int32(binary.LittleEndian.Uint32(data[off:])),
					VisOfs:   int32(binary.LittleEndian.Uint32(data[off+4:])),
					BoundsMin: [3]int16{
						int16(binary.LittleEndian.Uint16(data[off+8:])),
						int16(binary.LittleEndian.Uint16(data[off+10:])),
						int16(binary.LittleEndian.Uint16(data[off+12:])),
					},
					BoundsMax: [3]int16{
						int16(binary.LittleEndian.Uint16(data[off+14:])),
						int16(binary.LittleEndian.Uint16(data[off+16:])),
						int16(binary.LittleEndian.Uint16(data[off+18:])),
					},
					FirstMarkSurface: binary.LittleEndian.Uint16(data[off+20:]),
					NumMarkSurfaces:  binary.LittleEndian.Uint16(data[off+22:]),
					AmbientLevel:     [NumAmbients]uint8{data[off+24], data[off+25], data[off+26], data[off+27]},
				}
			}
			file.Leafs = leafs
		}
	}

	// 11: MarkSurfaces
	if data, err := reader.ReadLump(&file.Header.Lumps[LumpMarksurfaces]); err != nil {
		return nil, fmt.Errorf("load marksurfaces: %w", err)
	} else if len(data) > 0 {
		if file.IsBSP2 {
			const recSize = 4
			if len(data)%recSize != 0 {
				return nil, fmt.Errorf("marksurfaces lump funny size %d", len(data))
			}
			count := len(data) / recSize
			ms := make([]uint32, count)
			for i := 0; i < count; i++ {
				ms[i] = binary.LittleEndian.Uint32(data[i*4:])
			}
			file.MarkSurfaces = ms
		} else {
			const recSize = 2
			if len(data)%recSize != 0 {
				return nil, fmt.Errorf("marksurfaces lump funny size %d", len(data))
			}
			count := len(data) / recSize
			ms := make([]uint16, count)
			for i := 0; i < count; i++ {
				ms[i] = binary.LittleEndian.Uint16(data[i*2:])
			}
			file.MarkSurfaces = ms
		}
	}

	// 12: Edges
	if data, err := reader.ReadLump(&file.Header.Lumps[LumpEdges]); err != nil {
		return nil, fmt.Errorf("load edges: %w", err)
	} else if len(data) > 0 {
		if file.IsBSP2 {
			const recSize = 8
			if len(data)%recSize != 0 {
				return nil, fmt.Errorf("edges lump funny size %d", len(data))
			}
			count := len(data) / recSize
			edges := make([]DLEdge, count)
			for i := 0; i < count; i++ {
				off := i * recSize
				edges[i] = DLEdge{
					V: [2]uint32{
						binary.LittleEndian.Uint32(data[off:]),
						binary.LittleEndian.Uint32(data[off+4:]),
					},
				}
			}
			file.Edges = edges
		} else {
			const recSize = 4
			if len(data)%recSize != 0 {
				return nil, fmt.Errorf("edges lump funny size %d", len(data))
			}
			count := len(data) / recSize
			edges := make([]DSEdge, count)
			for i := 0; i < count; i++ {
				off := i * recSize
				edges[i] = DSEdge{
					V: [2]uint16{
						binary.LittleEndian.Uint16(data[off:]),
						binary.LittleEndian.Uint16(data[off+2:]),
					},
				}
			}
			file.Edges = edges
		}
	}

	// 13: Surfedges
	if data, err := reader.ReadLump(&file.Header.Lumps[LumpSurfedges]); err != nil {
		return nil, fmt.Errorf("load surfedges: %w", err)
	} else if len(data) > 0 {
		const recSize = 4
		if len(data)%recSize != 0 {
			return nil, fmt.Errorf("surfedges lump funny size %d", len(data))
		}
		count := len(data) / recSize
		file.Surfedges = make([]int32, count)
		for i := 0; i < count; i++ {
			file.Surfedges[i] = int32(binary.LittleEndian.Uint32(data[i*4:]))
		}
	}

	// 14: Models
	if data, err := reader.ReadLump(&file.Header.Lumps[LumpModels]); err != nil {
		return nil, fmt.Errorf("load models: %w", err)
	} else if len(data) > 0 {
		const recSize = 64
		if len(data)%recSize != 0 {
			return nil, fmt.Errorf("models lump funny size %d", len(data))
		}
		count := len(data) / recSize
		file.Models = make([]DModel, count)
		for i := 0; i < count; i++ {
			off := i * recSize
			var m DModel
			m.BoundsMin = types.Vec3{
				X: math.Float32frombits(binary.LittleEndian.Uint32(data[off:])),
				Y: math.Float32frombits(binary.LittleEndian.Uint32(data[off+4:])),
				Z: math.Float32frombits(binary.LittleEndian.Uint32(data[off+8:])),
			}
			m.BoundsMax = types.Vec3{
				X: math.Float32frombits(binary.LittleEndian.Uint32(data[off+12:])),
				Y: math.Float32frombits(binary.LittleEndian.Uint32(data[off+16:])),
				Z: math.Float32frombits(binary.LittleEndian.Uint32(data[off+20:])),
			}
			m.Origin = types.Vec3{
				X: math.Float32frombits(binary.LittleEndian.Uint32(data[off+24:])),
				Y: math.Float32frombits(binary.LittleEndian.Uint32(data[off+28:])),
				Z: math.Float32frombits(binary.LittleEndian.Uint32(data[off+32:])),
			}
			for h := 0; h < MaxMapHulls; h++ {
				m.HeadNode[h] = int32(binary.LittleEndian.Uint32(data[off+36+h*4:]))
			}
			m.VisLeafs = int32(binary.LittleEndian.Uint32(data[off+52:]))
			m.FirstFace = int32(binary.LittleEndian.Uint32(data[off+56:]))
			m.NumFaces = int32(binary.LittleEndian.Uint32(data[off+60:]))
			file.Models[i] = m
		}
	}

	return file, nil
}
