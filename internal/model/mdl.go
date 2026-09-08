package model

import (
	"github.com/darkliquid/ironwail-go/pkg/mdl"
)

const (
	MDLIdent      = mdl.MDLIdent
	MDLVersion    = mdl.MDLVersion
	MDLOnSeam     = mdl.MDLOnSeam
	MDLFacesFront = mdl.MDLFacesFront

	AliasSingle     = mdl.AliasSingle
	AliasGroup      = mdl.AliasGroup
	AliasSkinSingle = mdl.AliasSkinSingle
	AliasSkinGroup  = mdl.AliasSkinGroup
)

type (
	AliasFrameType = mdl.AliasFrameType
	AliasSkinType  = mdl.AliasSkinType

	MDLHeader      = mdl.Header
	STVert         = mdl.STVert
	DTriangle      = mdl.Triangle
	TriVertX       = mdl.TriVertX
	AliasFrameDesc = mdl.FrameDesc
	AliasSkinDesc  = mdl.SkinDesc
	AliasHeader    = mdl.File

	DAliasFrame        = mdl.DAliasFrame
	DAliasGroup        = mdl.DAliasGroup
	DAliasInterval     = mdl.DAliasInterval
	DAliasSkinGroup    = mdl.DAliasSkinGroup
	DAliasSkinInterval = mdl.DAliasSkinInterval
	DAliasFrameType    = mdl.DAliasFrameType
	DAliasSkinType     = mdl.DAliasSkinType

	MDLReader = mdl.Reader
)

var (
	NewMDLReader    = mdl.NewReader
	LoadMDL         = mdl.Load
	DecodeVertex    = mdl.DecodeVertex
	NormalsTable    = mdl.NormalsTable
	GetNormal       = mdl.GetNormal
	Float32FromBits = mdl.Float32FromBits
)
