package main

import (
	"log/slog"
	"math"

	qimage "github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

type csqcClipRect struct {
	enabled bool
	x       float32
	y       float32
	width   float32
	height  float32
}

type csqcDrawActivity struct {
	drew bool
}

func (a *csqcDrawActivity) mark() {
	if a != nil {
		a.drew = true
	}
}

func lookupCSQCPic(name string) *qimage.QPic {
	if g.Draw == nil {
		return nil
	}
	return g.Draw.GetPic(name)
}

func cacheCSQCPic(name string, flags uint32) *qimage.QPic {
	if g.CSQC != nil {
		g.CSQC.PrecachePic(name)
	}
	if g.Draw == nil {
		return nil
	}
	if flags&csqcPicFlagNoLoad != 0 {
		if g.Draw.IsPicCached(name) {
			return g.Draw.GetPic(name)
		}
		return nil
	}
	return lookupCSQCPic(name)
}

func nearestPaletteIndex(r, g, b float32, palette []byte) byte {
	if len(palette) < 3 {
		return 0
	}

	targetR := int(clampUnitFloat32(r)*255 + 0.5)
	targetG := int(clampUnitFloat32(g)*255 + 0.5)
	targetB := int(clampUnitFloat32(b)*255 + 0.5)

	bestIdx := 0
	bestDist := math.MaxInt
	for i := 0; i+2 < len(palette); i += 3 {
		dr := targetR - int(palette[i])
		dg := targetG - int(palette[i+1])
		db := targetB - int(palette[i+2])
		dist := dr*dr + dg*dg + db*db
		if dist < bestDist {
			bestDist = dist
			bestIdx = i / 3
		}
	}

	return byte(bestIdx)
}

func clipCSQCDrawRect(clip csqcClipRect, x, y, width, height float32) (drawX, drawY, drawW, drawH, srcX, srcY, srcW, srcH float32, ok bool) {
	if width <= 0 || height <= 0 {
		return 0, 0, 0, 0, 0, 0, 0, 0, false
	}

	drawX, drawY, drawW, drawH = x, y, width, height
	srcX, srcY, srcW, srcH = 0, 0, 1, 1
	if !clip.enabled {
		return drawX, drawY, drawW, drawH, srcX, srcY, srcW, srcH, true
	}

	left := max(x, clip.x)
	top := max(y, clip.y)
	right := min(x+width, clip.x+clip.width)
	bottom := min(y+height, clip.y+clip.height)
	if right <= left || bottom <= top {
		return 0, 0, 0, 0, 0, 0, 0, 0, false
	}

	drawX = left
	drawY = top
	drawW = right - left
	drawH = bottom - top
	srcX = (left - x) / width
	srcY = (top - y) / height
	srcW = drawW / width
	srcH = drawH / height
	return drawX, drawY, drawW, drawH, srcX, srcY, srcW, srcH, true
}

func subPicFromNormalizedRect(pic *qimage.QPic, srcX, srcY, srcW, srcH float32) *qimage.QPic {
	if pic == nil || pic.Width == 0 || pic.Height == 0 {
		return nil
	}

	startX := clampUnitFloat32(srcX)
	startY := clampUnitFloat32(srcY)
	endX := clampUnitFloat32(srcX + srcW)
	endY := clampUnitFloat32(srcY + srcH)
	if endX <= startX || endY <= startY {
		return &qimage.QPic{}
	}

	picWidth := float64(pic.Width)
	picHeight := float64(pic.Height)
	x1 := int(math.Floor(float64(startX) * picWidth))
	y1 := int(math.Floor(float64(startY) * picHeight))
	x2 := int(math.Ceil(float64(endX) * picWidth))
	y2 := int(math.Ceil(float64(endY) * picHeight))
	return pic.SubPic(x1, y1, x2-x1, y2-y1)
}

func scaleQPic(pic *qimage.QPic, width, height int) *qimage.QPic {
	if pic == nil || width <= 0 || height <= 0 || pic.Width == 0 || pic.Height == 0 {
		return nil
	}
	if int(pic.Width) == width && int(pic.Height) == height {
		return pic
	}

	srcW := int(pic.Width)
	srcH := int(pic.Height)
	scaled := &qimage.QPic{
		Width:  uint32(width),
		Height: uint32(height),
		Pixels: make([]byte, width*height),
	}
	for y := range height {
		srcY := y * srcH / height
		for x := range width {
			srcX := x * srcW / width
			scaled.Pixels[y*width+x] = pic.Pixels[srcY*srcW+srcX]
		}
	}
	return scaled
}

func prepareCSQCPic(pic *qimage.QPic, posX, posY, sizeX, sizeY, srcX, srcY, srcW, srcH float32, clip csqcClipRect) (int, int, *qimage.QPic, bool) {
	drawX, drawY, drawW, drawH, clipSrcX, clipSrcY, clipSrcW, clipSrcH, ok := clipCSQCDrawRect(clip, posX, posY, sizeX, sizeY)
	if !ok {
		return 0, 0, nil, false
	}

	srcX += srcW * clipSrcX
	srcY += srcH * clipSrcY
	srcW *= clipSrcW
	srcH *= clipSrcH

	subPic := subPicFromNormalizedRect(pic, srcX, srcY, srcW, srcH)
	if subPic == nil || subPic.Width == 0 || subPic.Height == 0 {
		return 0, 0, nil, false
	}

	drawPic := scaleQPic(subPic, int(drawW), int(drawH))
	if drawPic == nil || drawPic.Width == 0 || drawPic.Height == 0 {
		return 0, 0, nil, false
	}

	return int(drawX), int(drawY), drawPic, true
}

func buildCSQCDrawHooksWithActivity(rc renderer.RenderContext, activity *csqcDrawActivity) qc.CSQCDrawHooks {
	var clip csqcClipRect

	return qc.CSQCDrawHooks{
		IsCachedPic: func(name string) bool {
			if g.Draw == nil {
				return false
			}
			return g.Draw.IsPicCached(name)
		},
		PrecachePic: func(name string, flags int) string {
			if name == "" {
				return ""
			}
			pic := cacheCSQCPic(name, uint32(flags))
			if pic == nil && uint32(flags)&csqcPicFlagBlock != 0 {
				return ""
			}
			return name
		},
		GetImageSize: func(name string) (float32, float32) {
			pic := cacheCSQCPic(name, csqcPicFlagAuto)
			if pic == nil {
				return 0, 0
			}
			return float32(pic.Width), float32(pic.Height)
		},
		DrawCharacter: func(posX, posY float32, char int, sizeX, sizeY float32, r, g, b, alpha float32, drawflag int) {
			if alpha <= 0 {
				return
			}
			rc.DrawCharacter(int(posX), int(posY), char)
			activity.mark()
		},
		DrawString: func(posX, posY float32, text string, sizeX, sizeY float32, r, g, b, alpha float32, drawflag int, useColors bool) {
			if alpha <= 0 || text == "" {
				return
			}
			step := int(sizeX)
			if step <= 0 {
				step = 8
			}
			x := int(posX)
			for _, ch := range text {
				rc.DrawCharacter(x, int(posY), int(ch))
				x += step
			}
			activity.mark()
		},
		DrawPic: func(posX, posY float32, name string, sizeX, sizeY float32, r, g, b, alpha float32, drawflag int) {
			if alpha <= 0 {
				return
			}
			pic := cacheCSQCPic(name, csqcPicFlagAuto)
			if pic == nil {
				return
			}
			x, y, drawPic, ok := prepareCSQCPic(pic, posX, posY, sizeX, sizeY, 0, 0, 1, 1, clip)
			if !ok {
				return
			}
			rc.DrawPic(x, y, drawPic)
			activity.mark()
		},
		DrawFill: func(posX, posY float32, sizeX, sizeY float32, red, green, blue, alpha float32, drawflag int) {
			if alpha <= 0 {
				return
			}
			x, y, width, height, _, _, _, _, ok := clipCSQCDrawRect(clip, posX, posY, sizeX, sizeY)
			if !ok {
				return
			}
			var palette []byte
			if g.Draw != nil {
				palette = g.Draw.Palette()
			}
			color := nearestPaletteIndex(red, green, blue, palette)
			rc.DrawFill(int(x), int(y), int(width), int(height), color)
			activity.mark()
		},
		DrawSubPic: func(posX, posY float32, sizeX, sizeY float32, name string, srcX, srcY, srcW, srcH float32, r, g, b, alpha float32, drawflag int) {
			if alpha <= 0 {
				return
			}
			pic := cacheCSQCPic(name, csqcPicFlagAuto)
			if pic == nil {
				return
			}
			x, y, drawPic, ok := prepareCSQCPic(pic, posX, posY, sizeX, sizeY, srcX, srcY, srcW, srcH, clip)
			if !ok {
				return
			}
			rc.DrawPic(x, y, drawPic)
			activity.mark()
		},
		SetClipArea: func(x, y, width, height float32) {
			clip = csqcClipRect{enabled: true, x: x, y: y, width: width, height: height}
		},
		ResetClipArea: func() {
			clip = csqcClipRect{}
		},
		StringWidth: func(text string, useColors bool, fontSizeX, fontSizeY float32) float32 {
			var count float32
			for range text {
				count++
			}
			return count * fontSizeX
		},
	}
}

func buildCSQCDrawHooks(rc renderer.RenderContext) qc.CSQCDrawHooks {
	return buildCSQCDrawHooksWithActivity(rc, nil)
}

func wireCSQCDrawHooks(rc renderer.RenderContext) {
	qc.SetCSQCDrawHooks(buildCSQCDrawHooks(rc))
}

func buildCSQCFrameState() qc.CSQCFrameState {
	var state qc.CSQCFrameState
	if g.Host != nil {
		state.RealTime = float32(g.Host.RealTime())
		state.FrameTime = float32(g.Host.FrameTime())
	}
	if g.Client != nil {
		state.Time = float32(g.Client.Time)
		state.MaxClients = float32(g.Client.MaxClients)
		state.Intermission = float32(g.Client.Intermission)
		state.IntermissionTime = float32(g.Client.CompletedTime)
		if g.Client.ViewEntity > 0 {
			state.PlayerLocalEntNum = float32(g.Client.ViewEntity)
			state.PlayerLocalNum = float32(g.Client.ViewEntity - 1)
		}
		state.ViewAngles = g.Client.ViewAngles
		state.ClientCommandFrame = float32(g.Client.CommandSequence)
	}
	return state
}

func drawRuntimeCSQCHUD(rc renderer.RenderContext, showScores bool) bool {
	if rc == nil || g.CSQC == nil || !g.CSQC.IsLoaded() {
		return false
	}

	activity := &csqcDrawActivity{}
	rc.SetCanvas(renderer.CanvasCSQC)
	qc.SetCSQCDrawHooks(buildCSQCDrawHooksWithActivity(rc, activity))

	frameState := buildCSQCFrameState()
	canvas := rc.Canvas()
	virtW := canvas.Right - canvas.Left
	virtH := canvas.Bottom - canvas.Top
	drewHUD, err := g.CSQC.CallDrawHud(frameState, virtW, virtH, showScores)
	if err != nil {
		slog.Error("CSQC_DrawHud failed", "error", err)
		return false
	}
	if !drewHUD && !activity.drew {
		return false
	}
	if showScores && g.CSQC.HasDrawScores() {
		if err := g.CSQC.CallDrawScores(frameState, virtW, virtH, showScores); err != nil {
			slog.Error("CSQC_DrawScores failed", "error", err)
		}
	}
	return true
}

var runtimeDrawCSQCHUD = drawRuntimeCSQCHUD

func drawRuntimeHUDLayer(rc renderer.RenderContext, w, h int, telemetryState *runtimeTelemetryState) {
	if rc == nil || telemetryState == nil {
		return
	}

	showScores := g.ShowScores && g.Client != nil && g.Client.MaxClients > 1
	csqcDrewHUD := runtimeDrawCSQCHUD(rc, showScores)
	telemetryState.ViewRect = runtimeOverlayViewRect(w, h, csqcDrewHUD)
	if !csqcDrewHUD && g.HUD != nil {
		rc.SetCanvas(renderer.CanvasDefault)
		g.HUD.SetScreenSize(w, h)
		updateHUDFromServer()
		g.HUD.Draw(rc)
	}
}
