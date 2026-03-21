//go:build opengl || cgo
// +build opengl cgo

package renderer

import "github.com/ironwail/ironwail-go/internal/model"

type glSpriteModel = spriteRenderModel
type glSpriteFrame = spriteRenderFrame

func uploadSpriteModel(modelID string, spr *model.MSprite) *glSpriteModel {
	return buildSpriteRenderModel(modelID, spr)
}
