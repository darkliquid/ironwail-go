package game

import (
	"log/slog"

	qimage "github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/renderer"

	csqcimpl "github.com/darkliquid/ironwail-go/internal/game/csqc"
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

func (g *Game) lookupCSQCPic(name string) *qimage.QPic {
	if g.Draw == nil {
		return nil
	}
	return g.Draw.Pic(name)
}

func (g *Game) cacheCSQCPic(name string, flags uint32) *qimage.QPic {
	if g.CSQC != nil {
		g.CSQC.PrecachePic(name)
	}
	if g.Draw == nil {
		return nil
	}
	if flags&CSQCPicFlagNoLoad != 0 {
		if g.Draw.IsPicCached(name) {
			return g.Draw.Pic(name)
		}
		return nil
	}
	return g.lookupCSQCPic(name)
}

func (g *Game) nearestPaletteIndex(r, G, b float32, palette []byte) byte {
	return csqcimpl.NearestPaletteIndex(r, G, b, palette)
}

func (g *Game) clipCSQCDrawRect(clip csqcClipRect, x, y, width, height float32) (drawX, drawY, drawW, drawH, srcX, srcY, srcW, srcH float32, ok bool) {
	return csqcimpl.ClipDrawRect(csqcimpl.ClipRect{Enabled: clip.enabled, X: clip.x, Y: clip.y, Width: clip.width, Height: clip.height}, x, y, width, height)
}

func (g *Game) prepareCSQCPic(pic *qimage.QPic, posX, posY, sizeX, sizeY, srcX, srcY, srcW, srcH float32, clip csqcClipRect) (int, int, *qimage.QPic, bool) {
	return csqcimpl.PreparePic(pic, posX, posY, sizeX, sizeY, srcX, srcY, srcW, srcH, csqcimpl.ClipRect{Enabled: clip.enabled, X: clip.x, Y: clip.y, Width: clip.width, Height: clip.height})
}

func (g *Game) getCSQCCharPic(char int) *qimage.QPic {
	if g.Draw == nil {
		return nil
	}
	conchars := g.Draw.ConcharsData()
	if len(conchars) < 128*128 {
		return nil
	}

	char = char & 255
	// conchars is arranged as a 16x16 grid of 8x8 glyphs
	cx := (char % 16) * 8
	cy := (char / 16) * 8

	pic := &qimage.QPic{
		Width:  8,
		Height: 8,
		Pixels: make([]byte, 64),
	}

	for y := range 8 {
		srcIdx := (cy+y)*128 + cx
		dstIdx := y * 8
		copy(pic.Pixels[dstIdx:dstIdx+8], conchars[srcIdx:srcIdx+8])
	}
	return pic
}

func (g *Game) buildCSQCDrawHooksWithActivity(rc renderer.RenderContext, activity *csqcDrawActivity) qc.CSQCDrawHooks {
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
			pic := g.cacheCSQCPic(name, uint32(flags))
			if pic == nil && uint32(flags)&CSQCPicFlagBlock != 0 {
				return ""
			}
			return name
		},
		GetImageSize: func(name string) (float32, float32) {
			pic := g.cacheCSQCPic(name, CSQCPicFlagAuto)
			if pic == nil {
				return 0, 0
			}
			return float32(pic.Width), float32(pic.Height)
		},
		DrawCharacter: func(posX, posY float32, char int, sizeX, sizeY float32, r, G, b, alpha float32, drawflag int) {
			if alpha <= 0 {
				return
			}
			pic := g.getCSQCCharPic(char)
			if pic == nil {
				return
			}
			x, y, drawPic, ok := g.prepareCSQCPic(pic, posX, posY, sizeX, sizeY, 0, 0, 1, 1, clip)
			if !ok {
				return
			}
			if alphaDrawer, ok := rc.(interface {
				DrawPicAlpha(x, y int, pic *qimage.QPic, alpha float32)
			}); ok {
				alphaDrawer.DrawPicAlpha(x, y, drawPic, alpha)
			} else {
				rc.DrawPic(x, y, drawPic)
			}
			activity.mark()
		},
		DrawString: func(posX, posY float32, text string, sizeX, sizeY float32, r, G, b, alpha float32, drawflag int, useColors bool) {
			if alpha <= 0 || text == "" {
				return
			}
			step := int(sizeX)
			if step <= 0 {
				step = 8
			}
			x := int(posX)
			for _, ch := range text {
				pic := g.getCSQCCharPic(int(ch))
				if pic != nil {
					dx, dy, drawPic, ok := g.prepareCSQCPic(pic, float32(x), posY, sizeX, sizeY, 0, 0, 1, 1, clip)
					if ok {
						if alphaDrawer, ok := rc.(interface {
							DrawPicAlpha(x, y int, pic *qimage.QPic, alpha float32)
						}); ok {
							alphaDrawer.DrawPicAlpha(dx, dy, drawPic, alpha)
						} else {
							rc.DrawPic(dx, dy, drawPic)
						}
					}
				}
				x += step
			}
			activity.mark()
		},
		DrawPic: func(posX, posY float32, name string, sizeX, sizeY float32, r, G, b, alpha float32, drawflag int) {
			if alpha <= 0 {
				return
			}
			pic := g.cacheCSQCPic(name, CSQCPicFlagAuto)
			if pic == nil {
				return
			}
			x, y, drawPic, ok := g.prepareCSQCPic(pic, posX, posY, sizeX, sizeY, 0, 0, 1, 1, clip)
			if !ok {
				return
			}
			if alphaDrawer, ok := rc.(interface {
				DrawPicAlpha(x, y int, pic *qimage.QPic, alpha float32)
			}); ok {
				alphaDrawer.DrawPicAlpha(x, y, drawPic, alpha)
			} else {
				rc.DrawPic(x, y, drawPic)
			}
			activity.mark()
		},
		DrawFill: func(posX, posY float32, sizeX, sizeY float32, red, green, blue, alpha float32, drawflag int) {
			if alpha <= 0 {
				return
			}
			x, y, width, height, _, _, _, _, ok := g.clipCSQCDrawRect(clip, posX, posY, sizeX, sizeY)
			if !ok {
				return
			}
			var palette []byte
			if g.Draw != nil {
				palette = g.Draw.Palette()
			}
			color := g.nearestPaletteIndex(red, green, blue, palette)
			rc.DrawFillAlpha(int(x), int(y), int(width), int(height), color, alpha)
			activity.mark()
		},
		DrawSubPic: func(posX, posY float32, sizeX, sizeY float32, name string, srcX, srcY, srcW, srcH float32, r, G, b, alpha float32, drawflag int) {
			if alpha <= 0 {
				return
			}
			pic := g.cacheCSQCPic(name, CSQCPicFlagAuto)
			if pic == nil {
				return
			}
			// CSQC specs use pixel coordinates, but some legacy internal Go tests pass normalized [0, 1] bounds.
			// Proactively detect if coordinates are already normalized:
			isNormalized := srcX <= 1.0 && srcY <= 1.0 && srcW <= 1.0 && srcH <= 1.0 && srcW > 0 && srcH > 0
			var normX, normY, normW, normH float32
			if isNormalized {
				normX, normY, normW, normH = srcX, srcY, srcW, srcH
			} else if pic.Width > 0 && pic.Height > 0 {
				normX = srcX / float32(pic.Width)
				normY = srcY / float32(pic.Height)
				normW = srcW / float32(pic.Width)
				normH = srcH / float32(pic.Height)
			}
			x, y, drawPic, ok := g.prepareCSQCPic(pic, posX, posY, sizeX, sizeY, normX, normY, normW, normH, clip)
			if !ok {
				return
			}
			if alphaDrawer, ok := rc.(interface {
				DrawPicAlpha(x, y int, pic *qimage.QPic, alpha float32)
			}); ok {
				alphaDrawer.DrawPicAlpha(x, y, drawPic, alpha)
			} else {
				rc.DrawPic(x, y, drawPic)
			}
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

func (g *Game) buildCSQCDrawHooks(rc renderer.RenderContext) qc.CSQCDrawHooks {
	return g.buildCSQCDrawHooksWithActivity(rc, nil)
}

func (g *Game) buildCSQCFrameState() qc.CSQCFrameState {
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

func (g *Game) drawRuntimeCSQCHUD(rc renderer.RenderContext, showScores bool) bool {
	if rc == nil || g.CSQC == nil || !g.CSQC.IsLoaded() {
		return false
	}

	activity := &csqcDrawActivity{}
	rc.SetCanvas(renderer.CanvasCSQC)
	qc.SetCSQCDrawHooks(g.buildCSQCDrawHooksWithActivity(rc, activity))

	frameState := g.buildCSQCFrameState()
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

func (g *Game) drawRuntimeHUDLayer(rc renderer.RenderContext, w, h int, telemetryState *TelemetryState) {
	if rc == nil || telemetryState == nil {
		return
	}

	showScores := g.ShowScores && g.Client != nil && g.Client.MaxClients > 1
	csqcDrewHUD := g.drawRuntimeCSQCHUD(rc, showScores)
	telemetryState.ViewRect = g.runtimeOverlayViewRect(w, h, csqcDrewHUD)
	if !csqcDrewHUD && g.HUD != nil {
		rc.SetCanvas(renderer.CanvasDefault)
		g.HUD.SetScreenSize(w, h)
		g.updateHUDFromServer()
		g.HUD.Draw(rc)
	}
}

type gameCSQCHandler struct {
	game *Game
}

func (h *gameCSQCHandler) Init() error {
	if h.game == nil || h.game.CSQC == nil || !h.game.CSQC.IsLoaded() {
		return nil
	}
	engineVersion := float32(10000*VersionMajor + 100*VersionMinor + VersionPatch)
	return h.game.CSQC.CallInit("Ironwail", engineVersion)
}

func (h *gameCSQCHandler) Shutdown() error {
	if h.game == nil || h.game.CSQC == nil || !h.game.CSQC.IsLoaded() {
		return nil
	}
	return h.game.CSQC.CallShutdown()
}

func (h *gameCSQCHandler) ParseStuffCmd(cmd string) bool {
	if h.game == nil || h.game.CSQC == nil || !h.game.CSQC.IsLoaded() {
		return false
	}
	handled, err := h.game.CSQC.CallParseStuffCmd(cmd)
	if err != nil {
		slog.Error("CSQC_Parse_StuffCmd failed", "error", err)
		return false
	}
	return handled
}

func (h *gameCSQCHandler) EntUpdate(isNew bool) {
	if h.game == nil || h.game.CSQC == nil || !h.game.CSQC.IsLoaded() {
		return
	}
	if err := h.game.CSQC.CallEntUpdate(isNew); err != nil {
		slog.Error("CSQC_Ent_Update failed", "error", err)
	}
}
