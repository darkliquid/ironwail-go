// Package renderer provides a software (CPU) renderer for headless screenshot capture.
// SoftwareRenderer implements RenderContext using Go's standard image package,
// enabling screenshot output without a GPU or display.
package renderer

import (
	stdimage "image"
	"image/color"
	"sort"

	"github.com/ironwail/ironwail-go/internal/bsp"
	qimage "github.com/ironwail/ironwail-go/internal/image"
)

// SoftwareRenderer implements RenderContext using a CPU-side image buffer.
// It is used for headless screenshot capture without requiring a GPU.
type SoftwareRenderer struct {
	img     *stdimage.RGBA
	width   int
	height  int
	gamma   float32
	palette []byte
	canvas  CanvasState
}

// NewSoftwareRenderer creates a SoftwareRenderer for an image of the given dimensions.
// gamma and palette are optional; passing nil palette gives grayscale QPic rendering.
func NewSoftwareRenderer(width, height int, gamma float32, palette []byte) *SoftwareRenderer {
	img := stdimage.NewRGBA(stdimage.Rect(0, 0, width, height))
	return &SoftwareRenderer{
		img:     img,
		width:   width,
		height:  height,
		gamma:   gamma,
		palette: palette,
	}
}

// Image returns the rendered image buffer, ready for PNG encoding.
func (s *SoftwareRenderer) Image() *stdimage.RGBA {
	return s.img
}

// Clear fills the entire image with the given RGBA colour.
func (s *SoftwareRenderer) Clear(r, g, b, a float32) {
	c := color.RGBA{
		R: uint8(clampF(r) * 255),
		G: uint8(clampF(g) * 255),
		B: uint8(clampF(b) * 255),
		A: uint8(clampF(a) * 255),
	}
	pix := s.img.Pix
	for i := 0; i < len(pix); i += 4 {
		pix[i] = c.R
		pix[i+1] = c.G
		pix[i+2] = c.B
		pix[i+3] = c.A
	}
}

// DrawTriangle fills the screen with the given colour (test/fallback).
func (s *SoftwareRenderer) DrawTriangle(r, g, b, a float32) {
	s.Clear(r, g, b, a)
}

// SurfaceView returns nil (not applicable to software renderer).
func (s *SoftwareRenderer) SurfaceView() interface{} { return nil }

// Gamma returns the current gamma correction value.
func (s *SoftwareRenderer) Gamma() float32 { return s.gamma }

// SetCanvas switches the active 2D canvas coordinate system.
func (s *SoftwareRenderer) SetCanvas(ct CanvasType) { s.canvas.Type = ct }

// Canvas returns the current canvas state.
func (s *SoftwareRenderer) Canvas() CanvasState { return s.canvas }

// DrawPic blits a QPic image at a screen-space position using the stored palette.
func (s *SoftwareRenderer) DrawPic(x, y int, pic *qimage.QPic) {
	s.drawPicRect(screenPicRect(x, y, pic), pic)
}

// DrawMenuPic blits a QPic image in 320x200 menu-space coordinates.
func (s *SoftwareRenderer) DrawMenuPic(x, y int, pic *qimage.QPic) {
	s.drawPicRect(menuPicRect(s.width, s.height, x, y, pic), pic)
}

func (s *SoftwareRenderer) drawPicRect(rect picRect, pic *qimage.QPic) {
	if pic == nil || len(pic.Pixels) == 0 {
		return
	}
	rgba := ConvertPaletteToRGBA(pic.Pixels, s.palette)
	srcW, srcH := int(pic.Width), int(pic.Height)

	dstX := int(rect.x)
	dstY := int(rect.y)
	dstW := int(rect.w)
	dstH := int(rect.h)
	if dstW <= 0 || dstH <= 0 {
		return
	}

	for dy := 0; dy < dstH; dy++ {
		sy := dstY + dy
		if sy < 0 || sy >= s.height {
			continue
		}
		srcY := dy * srcH / dstH
		for dx := 0; dx < dstW; dx++ {
			sx := dstX + dx
			if sx < 0 || sx >= s.width {
				continue
			}
			srcX := dx * srcW / dstW
			off := (srcY*srcW + srcX) * 4
			if off+3 >= len(rgba) {
				continue
			}
			if rgba[off+3] == 0 {
				continue // transparent
			}
			s.img.SetRGBA(sx, sy, color.RGBA{rgba[off], rgba[off+1], rgba[off+2], rgba[off+3]})
		}
	}
}

// DrawFill fills a rectangle with a Quake palette colour.
func (s *SoftwareRenderer) DrawFill(x, y, w, h int, palIdx byte) {
	s.DrawFillAlpha(x, y, w, h, palIdx, 1)
}

// DrawFillAlpha fills a rectangle with a Quake palette colour and explicit alpha.
func (s *SoftwareRenderer) DrawFillAlpha(x, y, w, h int, palIdx byte, alpha float32) {
	if alpha <= 0 {
		return
	}
	if IsTransparentIndex(palIdx) {
		return
	}
	if alpha > 1 {
		alpha = 1
	}
	pr, pg, pb := GetPaletteColor(palIdx, s.palette)
	c := color.RGBA{pr, pg, pb, uint8(alpha * 255)}
	for dy := 0; dy < h; dy++ {
		sy := y + dy
		if sy < 0 || sy >= s.height {
			continue
		}
		for dx := 0; dx < w; dx++ {
			sx := x + dx
			if sx < 0 || sx >= s.width {
				continue
			}
			if alpha >= 1 {
				s.img.SetRGBA(sx, sy, c)
				continue
			}
			dst := s.img.RGBAAt(sx, sy)
			inv := 1 - alpha
			s.img.SetRGBA(sx, sy, color.RGBA{
				R: uint8(float32(c.R)*alpha + float32(dst.R)*inv),
				G: uint8(float32(c.G)*alpha + float32(dst.G)*inv),
				B: uint8(float32(c.B)*alpha + float32(dst.B)*inv),
				A: uint8(float32(c.A) + float32(dst.A)*inv),
			})
		}
	}
}

// DrawCharacter renders a single character as a small coloured box (placeholder).
func (s *SoftwareRenderer) DrawCharacter(x, y int, num int) {
	s.DrawFill(x, y, 8, 8, byte(num%255))
}

// DrawMenuCharacter renders a single character in menu-space coordinates.
func (s *SoftwareRenderer) DrawMenuCharacter(x, y int, num int) {
	scale, xOff, yOff := menuScale(s.width, s.height)
	menuX := int(float32(x)*scale + xOff)
	menuY := int(float32(y)*scale + yOff)
	menuSize := int(8 * scale)
	if menuSize <= 0 {
		return
	}
	s.DrawFill(menuX, menuY, menuSize, menuSize, byte(num%255))
}

// DrawBSPWorld renders the BSP world geometry as a flat-shaded top-down orthographic
// projection. Each face is filled with a colour derived from its plane normal,
// giving a floor-plan style overview of the map.
func (s *SoftwareRenderer) DrawBSPWorld(tree *bsp.Tree) {
	if tree == nil || len(tree.Models) == 0 || len(tree.Faces) == 0 {
		return
	}

	worldModel := tree.Models[0]

	// World bounding box in XY (top-down view)
	minX := worldModel.BoundsMin[0]
	maxX := worldModel.BoundsMax[0]
	minY := worldModel.BoundsMin[1]
	maxY := worldModel.BoundsMax[1]

	rangeX := maxX - minX
	rangeY := maxY - minY
	if rangeX < 1 || rangeY < 1 {
		return
	}

	// Scale to fit with margin, preserving aspect ratio
	const margin = 20
	drawW := float32(s.width - 2*margin)
	drawH := float32(s.height - 2*margin)
	scaleX := drawW / rangeX
	scaleY := drawH / rangeY
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}
	scaledW := rangeX * scale
	scaledH := rangeY * scale
	offsetX := float32(margin) + (drawW-scaledW)/2
	offsetY := float32(margin) + (drawH-scaledH)/2

	project := func(wx, wy float32) [2]int {
		sx := int(offsetX + (wx-minX)*scale)
		// Flip Y: Quake Y increases northward, screen Y increases downward
		sy := int(float32(s.height) - offsetY - (wy-minY)*scale)
		return [2]int{sx, sy}
	}

	numFaces := int(worldModel.NumFaces)
	firstFace := int(worldModel.FirstFace)

	for fi := 0; fi < numFaces; fi++ {
		faceIdx := firstFace + fi
		if faceIdx >= len(tree.Faces) {
			break
		}
		face := &tree.Faces[faceIdx]

		verts := s.bspFaceVerts2D(tree, face, project)
		if len(verts) < 3 {
			continue
		}

		r, g, b := s.bspFaceColor(tree, face)
		s.fillPolygon(verts, r, g, b)
	}
}

// bspFaceVerts2D extracts 2D projected screen-space vertices for a BSP face.
func (s *SoftwareRenderer) bspFaceVerts2D(tree *bsp.Tree, face *bsp.TreeFace, project func(x, y float32) [2]int) [][2]int {
	verts := make([][2]int, 0, face.NumEdges)
	for i := int32(0); i < face.NumEdges; i++ {
		surfEdgeIdx := int(face.FirstEdge) + int(i)
		if surfEdgeIdx >= len(tree.Surfedges) {
			continue
		}
		surfEdge := tree.Surfedges[surfEdgeIdx]

		var vertIdx uint32
		if surfEdge >= 0 {
			if int(surfEdge) >= len(tree.Edges) {
				continue
			}
			vertIdx = tree.Edges[surfEdge].V[0]
		} else {
			edgeIdx := -surfEdge
			if int(edgeIdx) >= len(tree.Edges) {
				continue
			}
			vertIdx = tree.Edges[edgeIdx].V[1]
		}

		if int(vertIdx) >= len(tree.Vertexes) {
			continue
		}
		v := tree.Vertexes[vertIdx].Point
		verts = append(verts, project(v[0], v[1]))
	}
	return verts
}

// bspFaceColor returns an RGB colour for a face based on its plane normal.
// Floor faces are warm stone, ceiling faces are dark, walls vary by orientation.
func (s *SoftwareRenderer) bspFaceColor(tree *bsp.Tree, face *bsp.TreeFace) (r, g, b uint8) {
	if int(face.PlaneNum) >= len(tree.Planes) {
		return 100, 100, 100
	}
	n := tree.Planes[face.PlaneNum].Normal
	if face.Side != 0 {
		n[0], n[1], n[2] = -n[0], -n[1], -n[2]
	}
	nz := n[2]
	absNZ := nz
	if absNZ < 0 {
		absNZ = -absNZ
	}
	if absNZ > 0.7 {
		if nz > 0 {
			return 160, 140, 100 // floor – warm stone
		}
		return 50, 45, 35 // ceiling – dark
	}
	// Wall: lighter toward X-facing, darker toward Y-facing
	absNX := n[0]
	if absNX < 0 {
		absNX = -absNX
	}
	brightness := 80 + int(absNX*80)
	if brightness > 200 {
		brightness = 200
	}
	return uint8(brightness), uint8(float32(brightness) * 0.88), uint8(float32(brightness) * 0.75)
}

// fillPolygon fills a convex (or nearly convex) polygon using a scanline algorithm.
func (s *SoftwareRenderer) fillPolygon(pts [][2]int, r, g, b uint8) {
	if len(pts) < 3 {
		return
	}

	minY, maxY := pts[0][1], pts[0][1]
	for _, p := range pts[1:] {
		if p[1] < minY {
			minY = p[1]
		}
		if p[1] > maxY {
			maxY = p[1]
		}
	}
	if minY < 0 {
		minY = 0
	}
	if maxY >= s.height {
		maxY = s.height - 1
	}
	if minY > maxY {
		return
	}

	c := color.RGBA{r, g, b, 255}
	n := len(pts)
	xs := make([]int, 0, 8)

	for y := minY; y <= maxY; y++ {
		xs = xs[:0]
		for i := 0; i < n; i++ {
			p1 := pts[i]
			p2 := pts[(i+1)%n]
			y1, y2 := p1[1], p2[1]
			if (y1 <= y && y2 > y) || (y2 <= y && y1 > y) {
				x := p1[0] + (y-y1)*(p2[0]-p1[0])/(y2-y1)
				xs = append(xs, x)
			}
		}
		if len(xs) < 2 {
			continue
		}
		sort.Ints(xs)
		for i := 0; i+1 < len(xs); i += 2 {
			x1, x2 := xs[i], xs[i+1]
			if x1 < 0 {
				x1 = 0
			}
			if x2 >= s.width {
				x2 = s.width - 1
			}
			for x := x1; x <= x2; x++ {
				s.img.SetRGBA(x, y, c)
			}
		}
	}
}

// clampF clamps f to [0,1].
func clampF(f float32) float32 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}
