// Package bsp provides reading and parsing of Quake BSP files.
// It is now an alias and compatibility wrapper over the public pkg/bsp package.
package bsp

import (
	"github.com/darkliquid/ironwail-go/pkg/bsp"
)

// BSP version constants
const (
	BSPVersion         = bsp.BSPVersion
	BSP2Version_2PSB   = bsp.BSP2Version_2PSB
	BSP2Version_BSP2   = bsp.BSP2Version_BSP2
	BSPVersion_Quake64 = bsp.BSPVersion_Quake64
)

// Lump type indices
const (
	LumpEntities     = bsp.LumpEntities
	LumpPlanes       = bsp.LumpPlanes
	LumpTextures     = bsp.LumpTextures
	LumpVertexes     = bsp.LumpVertexes
	LumpVisibility   = bsp.LumpVisibility
	LumpNodes        = bsp.LumpNodes
	LumpTexinfo      = bsp.LumpTexinfo
	LumpFaces        = bsp.LumpFaces
	LumpLighting     = bsp.LumpLighting
	LumpClipnodes    = bsp.LumpClipnodes
	LumpLeafs        = bsp.LumpLeafs
	LumpMarksurfaces = bsp.LumpMarksurfaces
	LumpEdges        = bsp.LumpEdges
	LumpSurfedges    = bsp.LumpSurfedges
	LumpModels       = bsp.LumpModels
	HeaderLumps      = bsp.HeaderLumps
)

// Maximum map limits
const (
	MaxMapHulls        = bsp.MaxMapHulls
	MaxMapModels       = bsp.MaxMapModels
	MaxMapBrushes      = bsp.MaxMapBrushes
	MaxMapEntities     = bsp.MaxMapEntities
	MaxMapEntstring    = bsp.MaxMapEntstring
	MaxMapPlanes       = bsp.MaxMapPlanes
	MaxMapNodes        = bsp.MaxMapNodes
	MaxMapClipnodes    = bsp.MaxMapClipnodes
	MaxMapVerts        = bsp.MaxMapVerts
	MaxMapFaces        = bsp.MaxMapFaces
	MaxMapMarksurfaces = bsp.MaxMapMarksurfaces
	MaxMapTexinfo      = bsp.MaxMapTexinfo
	MaxMapEdges        = bsp.MaxMapEdges
	MaxMapSurfedges    = bsp.MaxMapSurfedges
	MaxMapTextures     = bsp.MaxMapTextures
	MaxMapMiptex       = bsp.MaxMapMiptex
	MaxMapLighting     = bsp.MaxMapLighting
	MaxMapVisibility   = bsp.MaxMapVisibility
	MaxMapPortals      = bsp.MaxMapPortals
)

// Entity key/value pair limits
const (
	MaxKey   = bsp.MaxKey
	MaxValue = bsp.MaxValue
)

// Mipmap levels
const (
	MipLevels = bsp.MipLevels
)

// Content types for leafs
const (
	ContentsEmpty  = bsp.ContentsEmpty
	ContentsSolid  = bsp.ContentsSolid
	ContentsWater  = bsp.ContentsWater
	ContentsSlime  = bsp.ContentsSlime
	ContentsLava   = bsp.ContentsLava
	ContentsSky    = bsp.ContentsSky
	ContentsOrigin = bsp.ContentsOrigin
	ContentsClip   = bsp.ContentsClip

	ContentsCurrent0    = bsp.ContentsCurrent0
	ContentsCurrent90   = bsp.ContentsCurrent90
	ContentsCurrent180  = bsp.ContentsCurrent180
	ContentsCurrent270  = bsp.ContentsCurrent270
	ContentsCurrentUp   = bsp.ContentsCurrentUp
	ContentsCurrentDown = bsp.ContentsCurrentDown
)

// Plane types
const (
	PlaneX    = bsp.PlaneX
	PlaneY    = bsp.PlaneY
	PlaneZ    = bsp.PlaneZ
	PlaneAnyX = bsp.PlaneAnyX
	PlaneAnyY = bsp.PlaneAnyY
	PlaneAnyZ = bsp.PlaneAnyZ
)

// Ambient sound indices
const (
	AmbientWater = bsp.AmbientWater
	AmbientSky   = bsp.AmbientSky
	AmbientSlime = bsp.AmbientSlime
	AmbientLava  = bsp.AmbientLava
	NumAmbients  = bsp.NumAmbients
)

// MaxLightmaps is the maximum number of lightmap styles per face
const MaxLightmaps = bsp.MaxLightmaps

// Texinfo flags
const (
	TexSpecial = bsp.TexSpecial
	TexMissing = bsp.TexMissing
)

// Type aliases to pkg/bsp
type (
	Lump         = bsp.Lump
	DHeader      = bsp.DHeader
	DModel       = bsp.DModel
	DMiptexLump  = bsp.DMiptexLump
	Miptex       = bsp.Miptex
	Miptex64     = bsp.Miptex64
	DVertex      = bsp.DVertex
	DPlane       = bsp.DPlane
	DSNode       = bsp.DSNode
	DL1Node      = bsp.DL1Node
	DL2Node      = bsp.DL2Node
	DSClipNode   = bsp.DSClipNode
	DLClipNode   = bsp.DLClipNode
	Texinfo      = bsp.Texinfo
	DSEdge       = bsp.DSEdge
	DLEdge       = bsp.DLEdge
	DSFace       = bsp.DSFace
	DLFace       = bsp.DLFace
	DSLeaf       = bsp.DSLeaf
	DL1Leaf      = bsp.DL1Leaf
	DL2Leaf      = bsp.DL2Leaf
	File             = bsp.File
	Reader           = bsp.Reader
	Entity           = bsp.Entity
	Portal           = bsp.Portal
	PortalFile       = bsp.PortalFile
)

var (
	NewReader        = bsp.NewReader
	IsBSP2           = bsp.IsBSP2
	IsQuake64        = bsp.IsQuake64
	IsValidVersion   = bsp.IsValidVersion
	ReadLumps        = bsp.ReadLumps
	WriteLumps       = bsp.WriteLumps
	WriteBSP         = bsp.WriteBSP
	PatchLump        = bsp.PatchLump
	ParseEntities    = bsp.ParseEntities
	ParseFirstEntity = bsp.ParseFirstEntity
	ParsePortalFile  = bsp.ParsePortalFile
)
