package model

import (
	"github.com/darkliquid/ironwail-go/pkg/mdl"
)

const (
	IDSpriteHeader = mdl.IDSpriteHeader
	SpriteVersion  = mdl.SpriteVersion

	SpriteFrameSingle = mdl.SpriteFrameSingle
	SpriteFrameGroup  = mdl.SpriteFrameGroup
	SpriteFrameAngled = mdl.SpriteFrameAngled
)

type (
	MSprite          = mdl.Sprite
	MSpriteFrame     = mdl.SpriteFrame
	MSpriteGroup     = mdl.SpriteGroup
	MSpriteFrameDesc = mdl.SpriteFrameDesc
)

var LoadSprite = mdl.LoadSprite
