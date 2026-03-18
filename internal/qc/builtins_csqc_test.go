package qc

import "testing"

func TestCSQCDrawBuiltinsNoHooks(t *testing.T) {
	SetCSQCDrawHooks(CSQCDrawHooks{})
	defer SetCSQCDrawHooks(CSQCDrawHooks{})

	vm := newBuiltinsTestVM(4)

	vm.SetGString(OFSParm0, "gfx/wad/conback")
	csqcIsCachedPic(vm)
	if got := vm.GFloat(OFSReturn); got != 0 {
		t.Fatalf("iscachedpic = %v, want 0", got)
	}

	vm.SetGString(OFSParm0, "gfx/wad/conback")
	vm.SetGFloat(OFSParm1, 1)
	csqcPrecachePic(vm)
	if got := vm.GString(OFSReturn); got != "" {
		t.Fatalf("precache_pic = %q, want empty", got)
	}

	csqcDrawGetImageSize(vm)
	if got := vm.GVector(OFSReturn); got != [3]float32{} {
		t.Fatalf("drawgetimagesize = %v, want zero", got)
	}

	csqcDrawCharacter(vm)
	if got := vm.GFloat(OFSReturn); got != 0 {
		t.Fatalf("drawcharacter = %v, want 0", got)
	}

	csqcDrawRawString(vm)
	if got := vm.GFloat(OFSReturn); got != 0 {
		t.Fatalf("drawrawstring = %v, want 0", got)
	}

	csqcDrawPic(vm)
	if got := vm.GFloat(OFSReturn); got != 0 {
		t.Fatalf("drawpic = %v, want 0", got)
	}

	csqcDrawFill(vm)
	if got := vm.GFloat(OFSReturn); got != 0 {
		t.Fatalf("drawfill = %v, want 0", got)
	}

	csqcStringWidth(vm)
	if got := vm.GFloat(OFSReturn); got != 0 {
		t.Fatalf("stringwidth = %v, want 0", got)
	}

	csqcDrawSetClipArea(vm)
	csqcDrawResetClipArea(vm)
	csqcDrawSubPic(vm)
}

func TestCSQCDrawBuiltinsUseHooks(t *testing.T) {
	SetCSQCDrawHooks(CSQCDrawHooks{})
	defer SetCSQCDrawHooks(CSQCDrawHooks{})

	vm := newBuiltinsTestVM(4)

	var drawStringUseColors []bool
	var gotDrawSubPic struct {
		name         string
		posX, posY   float32
		sizeX, sizeY float32
		srcX, srcY   float32
		srcW, srcH   float32
		r, g, b      float32
		alpha        float32
		drawflag     int
	}
	setClipCalls := 0
	resetClipCalls := 0
	SetCSQCDrawHooks(CSQCDrawHooks{
		IsCachedPic: func(name string) bool {
			return name == "cached"
		},
		PrecachePic: func(name string, flags int) string {
			if name == "pic" && flags == 7 {
				return "pic"
			}
			return ""
		},
		GetImageSize: func(name string) (width, height float32) {
			if name == "img" {
				return 320, 200
			}
			return 0, 0
		},
		DrawCharacter: func(posX, posY float32, char int, sizeX, sizeY float32, r, g, b, alpha float32, drawflag int) {
			if posX != 1 || posY != 2 || char != 65 || sizeX != 8 || sizeY != 8 || r != 0.1 || g != 0.2 || b != 0.3 || alpha != 0.4 || drawflag != 5 {
				t.Fatalf("drawcharacter args mismatch")
			}
		},
		DrawString: func(posX, posY float32, text string, sizeX, sizeY float32, r, g, b, alpha float32, drawflag int, useColors bool) {
			if posX != 11 || posY != 12 || text != "hello" || sizeX != 9 || sizeY != 10 || r != 0.7 || g != 0.8 || b != 0.9 || alpha != 0.6 || drawflag != 3 {
				t.Fatalf("drawstring args mismatch")
			}
			drawStringUseColors = append(drawStringUseColors, useColors)
		},
		DrawPic: func(posX, posY float32, name string, sizeX, sizeY float32, r, g, b, alpha float32, drawflag int) {
			if posX != 21 || posY != 22 || name != "hud" || sizeX != 100 || sizeY != 50 || r != 0.1 || g != 0.2 || b != 0.3 || alpha != 0.4 || drawflag != 2 {
				t.Fatalf("drawpic args mismatch")
			}
		},
		DrawFill: func(posX, posY float32, sizeX, sizeY float32, r, g, b, alpha float32, drawflag int) {
			if posX != 31 || posY != 32 || sizeX != 200 || sizeY != 100 || r != 1 || g != 0.5 || b != 0.25 || alpha != 0.75 || drawflag != 4 {
				t.Fatalf("drawfill args mismatch")
			}
		},
		DrawSubPic: func(posX, posY float32, sizeX, sizeY float32, name string, srcX, srcY, srcW, srcH float32, r, g, b, alpha float32, drawflag int) {
			gotDrawSubPic = struct {
				name         string
				posX, posY   float32
				sizeX, sizeY float32
				srcX, srcY   float32
				srcW, srcH   float32
				r, g, b      float32
				alpha        float32
				drawflag     int
			}{
				name:     name,
				posX:     posX,
				posY:     posY,
				sizeX:    sizeX,
				sizeY:    sizeY,
				srcX:     srcX,
				srcY:     srcY,
				srcW:     srcW,
				srcH:     srcH,
				r:        r,
				g:        g,
				b:        b,
				alpha:    alpha,
				drawflag: drawflag,
			}
		},
		SetClipArea: func(x, y, width, height float32) {
			setClipCalls++
			if x != 10 || y != 20 || width != 30 || height != 40 {
				t.Fatalf("setcliparea args mismatch")
			}
		},
		ResetClipArea: func() {
			resetClipCalls++
		},
		StringWidth: func(text string, useColors bool, fontSizeX, fontSizeY float32) float32 {
			if text != "wide" || !useColors || fontSizeX != 8 || fontSizeY != 12 {
				t.Fatalf("stringwidth args mismatch")
			}
			return 42
		},
	})

	vm.SetGString(OFSParm0, "cached")
	csqcIsCachedPic(vm)
	if got := vm.GFloat(OFSReturn); got != 1 {
		t.Fatalf("iscachedpic = %v, want 1", got)
	}

	vm.SetGString(OFSParm0, "pic")
	vm.SetGFloat(OFSParm1, 7)
	csqcPrecachePic(vm)
	if got := vm.GString(OFSReturn); got != "pic" {
		t.Fatalf("precache_pic = %q, want pic", got)
	}

	vm.SetGString(OFSParm0, "img")
	csqcDrawGetImageSize(vm)
	if got := vm.GVector(OFSReturn); got != [3]float32{320, 200, 0} {
		t.Fatalf("drawgetimagesize = %v, want 320x200", got)
	}

	vm.SetGVector(OFSParm0, [3]float32{1, 2, 0})
	vm.SetGFloat(OFSParm1, 65)
	vm.SetGVector(OFSParm2, [3]float32{8, 8, 0})
	vm.SetGVector(OFSParm3, [3]float32{0.1, 0.2, 0.3})
	vm.SetGFloat(OFSParm4, 0.4)
	vm.SetGFloat(OFSParm5, 5)
	csqcDrawCharacter(vm)
	if got := vm.GFloat(OFSReturn); got != 1 {
		t.Fatalf("drawcharacter = %v, want 1", got)
	}

	vm.SetGVector(OFSParm0, [3]float32{11, 12, 0})
	vm.SetGString(OFSParm1, "hello")
	vm.SetGVector(OFSParm2, [3]float32{9, 10, 0})
	vm.SetGVector(OFSParm3, [3]float32{0.7, 0.8, 0.9})
	vm.SetGFloat(OFSParm4, 0.6)
	vm.SetGFloat(OFSParm5, 3)
	csqcDrawRawString(vm)
	if got := vm.GFloat(OFSReturn); got != 1 {
		t.Fatalf("drawrawstring = %v, want 1", got)
	}
	csqcDrawString(vm)
	if got := vm.GFloat(OFSReturn); got != 1 {
		t.Fatalf("drawstring = %v, want 1", got)
	}

	vm.SetGVector(OFSParm0, [3]float32{21, 22, 0})
	vm.SetGString(OFSParm1, "hud")
	vm.SetGVector(OFSParm2, [3]float32{100, 50, 0})
	vm.SetGVector(OFSParm3, [3]float32{0.1, 0.2, 0.3})
	vm.SetGFloat(OFSParm4, 0.4)
	vm.SetGFloat(OFSParm5, 2)
	csqcDrawPic(vm)
	if got := vm.GFloat(OFSReturn); got != 1 {
		t.Fatalf("drawpic = %v, want 1", got)
	}

	vm.SetGVector(OFSParm0, [3]float32{31, 32, 0})
	vm.SetGVector(OFSParm1, [3]float32{200, 100, 0})
	vm.SetGVector(OFSParm2, [3]float32{1, 0.5, 0.25})
	vm.SetGFloat(OFSParm3, 0.75)
	vm.SetGFloat(OFSParm4, 4)
	csqcDrawFill(vm)
	if got := vm.GFloat(OFSReturn); got != 1 {
		t.Fatalf("drawfill = %v, want 1", got)
	}

	vm.SetGFloat(OFSParm0, 10)
	vm.SetGFloat(OFSParm1, 20)
	vm.SetGFloat(OFSParm2, 30)
	vm.SetGFloat(OFSParm3, 40)
	csqcDrawSetClipArea(vm)
	if setClipCalls != 1 {
		t.Fatalf("setcliparea calls = %d, want 1", setClipCalls)
	}

	csqcDrawResetClipArea(vm)
	if resetClipCalls != 1 {
		t.Fatalf("resetcliparea calls = %d, want 1", resetClipCalls)
	}

	vm.SetGString(OFSParm0, "wide")
	vm.SetGFloat(OFSParm1, 1)
	vm.SetGVector(OFSParm2, [3]float32{8, 12, 0})
	csqcStringWidth(vm)
	if got := vm.GFloat(OFSReturn); got != 42 {
		t.Fatalf("stringwidth = %v, want 42", got)
	}

	vm.SetGVector(OFSParm0, [3]float32{50, 60, 0})
	vm.SetGVector(OFSParm1, [3]float32{70, 80, 0})
	vm.SetGString(OFSParm2, "sub")
	vm.SetGVector(OFSParm3, [3]float32{1, 2, 0})
	vm.SetGVector(OFSParm4, [3]float32{3, 4, 0})
	vm.SetGVector(OFSParm5, [3]float32{0.25, 0.5, 0.75})
	vm.SetGFloat(OFSParm6, 0.9)
	vm.SetGFloat(OFSParm7, 8)
	csqcDrawSubPic(vm)

	if gotDrawSubPic.name != "sub" || gotDrawSubPic.posX != 50 || gotDrawSubPic.posY != 60 ||
		gotDrawSubPic.sizeX != 70 || gotDrawSubPic.sizeY != 80 || gotDrawSubPic.srcX != 1 || gotDrawSubPic.srcY != 2 ||
		gotDrawSubPic.srcW != 3 || gotDrawSubPic.srcH != 4 || gotDrawSubPic.r != 0.25 || gotDrawSubPic.g != 0.5 ||
		gotDrawSubPic.b != 0.75 || gotDrawSubPic.alpha != 0.9 || gotDrawSubPic.drawflag != 8 {
		t.Fatalf("drawsubpic args mismatch: %+v", gotDrawSubPic)
	}

	if len(drawStringUseColors) != 2 || drawStringUseColors[0] || !drawStringUseColors[1] {
		t.Fatalf("drawstring useColors calls = %v, want [false true]", drawStringUseColors)
	}
}
