package qc

// CSQCDrawHooks provides drawing operations for CSQC builtins.
// The renderer implements this interface and registers it via SetCSQCDrawHooks.
type CSQCDrawHooks struct {
	// IsCachedPic checks whether a pic is already cached.
	IsCachedPic func(name string) bool

	// PrecachePic loads a pic by name and returns the name (or "" on failure).
	PrecachePic func(name string, flags int) string

	// GetImageSize returns the dimensions of a named pic.
	GetImageSize func(name string) (width, height float32)

	// DrawCharacter draws a single character.
	// pos: screen position, char: character code, size: character dimensions,
	// rgb: color tint, alpha: opacity, drawflag: blend mode.
	DrawCharacter func(posX, posY float32, char int, sizeX, sizeY float32, r, g, b, alpha float32, drawflag int)

	// DrawString draws a text string (with color code processing if useColors is true).
	DrawString func(posX, posY float32, text string, sizeX, sizeY float32, r, g, b, alpha float32, drawflag int, useColors bool)

	// DrawPic draws a named pic at a position.
	DrawPic func(posX, posY float32, name string, sizeX, sizeY float32, r, g, b, alpha float32, drawflag int)

	// DrawFill draws a filled rectangle.
	DrawFill func(posX, posY float32, sizeX, sizeY float32, r, g, b, alpha float32, drawflag int)

	// DrawSubPic draws a sub-region of a named pic.
	DrawSubPic func(posX, posY float32, sizeX, sizeY float32, name string, srcX, srcY, srcW, srcH float32, r, g, b, alpha float32, drawflag int)

	// SetClipArea sets the scissor rectangle for drawing.
	SetClipArea func(x, y, width, height float32)

	// ResetClipArea removes the scissor rectangle.
	ResetClipArea func()

	// StringWidth measures the width of text at the given font size.
	StringWidth func(text string, useColors bool, fontSizeX, fontSizeY float32) float32
}

var csqcDrawHooks CSQCDrawHooks

// SetCSQCDrawHooks registers the drawing hooks for CSQC builtins.
func SetCSQCDrawHooks(hooks CSQCDrawHooks) {
	csqcDrawHooks = hooks
}

// csqcIsCachedPic implements CSQC builtin 316: iscachedpic.
// float iscachedpic(string name)
func csqcIsCachedPic(vm *VM) {
	if csqcDrawHooks.IsCachedPic == nil {
		vm.SetGFloat(OFSReturn, 0)
		return
	}
	if csqcDrawHooks.IsCachedPic(vm.GString(OFSParm0)) {
		vm.SetGFloat(OFSReturn, 1)
		return
	}
	vm.SetGFloat(OFSReturn, 0)
}

// csqcPrecachePic implements CSQC builtin 317: precache_pic.
// string precache_pic(string name, float flags)
func csqcPrecachePic(vm *VM) {
	if csqcDrawHooks.PrecachePic == nil {
		vm.SetGString(OFSReturn, "")
		return
	}
	name := vm.GString(OFSParm0)
	flags := int(vm.GFloat(OFSParm1))
	vm.SetGString(OFSReturn, csqcDrawHooks.PrecachePic(name, flags))
}

// csqcDrawGetImageSize implements CSQC builtin 318: drawgetimagesize.
// vector drawgetimagesize(string name)
func csqcDrawGetImageSize(vm *VM) {
	if csqcDrawHooks.GetImageSize == nil {
		vm.SetGVector(OFSReturn, [3]float32{0, 0, 0})
		return
	}
	width, height := csqcDrawHooks.GetImageSize(vm.GString(OFSParm0))
	vm.SetGVector(OFSReturn, [3]float32{width, height, 0})
}

// csqcDrawCharacter implements CSQC builtin 320: drawcharacter.
// float drawcharacter(vector pos, float char, vector size, vector rgb, float alpha, float drawflag)
func csqcDrawCharacter(vm *VM) {
	if csqcDrawHooks.DrawCharacter == nil {
		vm.SetGFloat(OFSReturn, 0)
		return
	}
	pos := vm.GVector(OFSParm0)
	ch := int(vm.GFloat(OFSParm1))
	size := vm.GVector(OFSParm2)
	rgb := vm.GVector(OFSParm3)
	alpha := vm.GFloat(OFSParm4)
	drawflag := int(vm.GFloat(OFSParm5))
	csqcDrawHooks.DrawCharacter(pos[0], pos[1], ch, size[0], size[1], rgb[0], rgb[1], rgb[2], alpha, drawflag)
	vm.SetGFloat(OFSReturn, 1)
}

// csqcDrawRawString implements CSQC builtin 321: drawrawstring.
// float drawrawstring(vector pos, string text, vector size, vector rgb, float alpha, float drawflag)
func csqcDrawRawString(vm *VM) {
	if csqcDrawHooks.DrawString == nil {
		vm.SetGFloat(OFSReturn, 0)
		return
	}
	pos := vm.GVector(OFSParm0)
	text := vm.GString(OFSParm1)
	size := vm.GVector(OFSParm2)
	rgb := vm.GVector(OFSParm3)
	alpha := vm.GFloat(OFSParm4)
	drawflag := int(vm.GFloat(OFSParm5))
	csqcDrawHooks.DrawString(pos[0], pos[1], text, size[0], size[1], rgb[0], rgb[1], rgb[2], alpha, drawflag, false)
	vm.SetGFloat(OFSReturn, 1)
}

// csqcDrawPic implements CSQC builtin 322: drawpic.
// float drawpic(vector pos, string pic, vector size, vector rgb, float alpha, float drawflag)
func csqcDrawPic(vm *VM) {
	if csqcDrawHooks.DrawPic == nil {
		vm.SetGFloat(OFSReturn, 0)
		return
	}
	pos := vm.GVector(OFSParm0)
	name := vm.GString(OFSParm1)
	size := vm.GVector(OFSParm2)
	rgb := vm.GVector(OFSParm3)
	alpha := vm.GFloat(OFSParm4)
	drawflag := int(vm.GFloat(OFSParm5))
	csqcDrawHooks.DrawPic(pos[0], pos[1], name, size[0], size[1], rgb[0], rgb[1], rgb[2], alpha, drawflag)
	vm.SetGFloat(OFSReturn, 1)
}

// csqcDrawFill implements CSQC builtin 323: drawfill.
// float drawfill(vector pos, vector size, vector rgb, float alpha, float drawflag)
func csqcDrawFill(vm *VM) {
	if csqcDrawHooks.DrawFill == nil {
		vm.SetGFloat(OFSReturn, 0)
		return
	}
	pos := vm.GVector(OFSParm0)
	size := vm.GVector(OFSParm1)
	rgb := vm.GVector(OFSParm2)
	alpha := vm.GFloat(OFSParm3)
	drawflag := int(vm.GFloat(OFSParm4))
	csqcDrawHooks.DrawFill(pos[0], pos[1], size[0], size[1], rgb[0], rgb[1], rgb[2], alpha, drawflag)
	vm.SetGFloat(OFSReturn, 1)
}

// csqcDrawSetClipArea implements CSQC builtin 324: drawsetcliparea.
// void drawsetcliparea(float x, float y, float width, float height)
func csqcDrawSetClipArea(vm *VM) {
	if csqcDrawHooks.SetClipArea == nil {
		return
	}
	x := vm.GFloat(OFSParm0)
	y := vm.GFloat(OFSParm1)
	width := vm.GFloat(OFSParm2)
	height := vm.GFloat(OFSParm3)
	csqcDrawHooks.SetClipArea(x, y, width, height)
}

// csqcDrawResetClipArea implements CSQC builtin 325: drawresetcliparea.
// void drawresetcliparea()
func csqcDrawResetClipArea(vm *VM) {
	if csqcDrawHooks.ResetClipArea == nil {
		return
	}
	csqcDrawHooks.ResetClipArea()
}

// csqcDrawString implements CSQC builtin 326: drawstring.
// float drawstring(vector pos, string text, vector size, vector rgb, float alpha, float drawflag)
func csqcDrawString(vm *VM) {
	if csqcDrawHooks.DrawString == nil {
		vm.SetGFloat(OFSReturn, 0)
		return
	}
	pos := vm.GVector(OFSParm0)
	text := vm.GString(OFSParm1)
	size := vm.GVector(OFSParm2)
	rgb := vm.GVector(OFSParm3)
	alpha := vm.GFloat(OFSParm4)
	drawflag := int(vm.GFloat(OFSParm5))
	csqcDrawHooks.DrawString(pos[0], pos[1], text, size[0], size[1], rgb[0], rgb[1], rgb[2], alpha, drawflag, true)
	vm.SetGFloat(OFSReturn, 1)
}

// csqcStringWidth implements CSQC builtin 327: stringwidth.
// float stringwidth(string text, float usecolours, vector fontsize)
func csqcStringWidth(vm *VM) {
	if csqcDrawHooks.StringWidth == nil {
		vm.SetGFloat(OFSReturn, 0)
		return
	}
	text := vm.GString(OFSParm0)
	useColors := vm.GFloat(OFSParm1) != 0
	fontSize := vm.GVector(OFSParm2)
	width := csqcDrawHooks.StringWidth(text, useColors, fontSize[0], fontSize[1])
	vm.SetGFloat(OFSReturn, width)
}

// csqcDrawSubPic implements CSQC builtin 328: drawsubpic.
// void drawsubpic(vector pos, vector sz, string pic, vector srcpos, vector srcsz, vector rgb, float alpha, float drawflag)
func csqcDrawSubPic(vm *VM) {
	if csqcDrawHooks.DrawSubPic == nil {
		return
	}
	pos := vm.GVector(OFSParm0)
	size := vm.GVector(OFSParm1)
	name := vm.GString(OFSParm2)
	srcPos := vm.GVector(OFSParm3)
	srcSize := vm.GVector(OFSParm4)
	rgb := vm.GVector(OFSParm5)
	alpha := vm.GFloat(OFSParm6)
	drawflag := int(vm.GFloat(OFSParm7))
	csqcDrawHooks.DrawSubPic(
		pos[0], pos[1], size[0], size[1], name,
		srcPos[0], srcPos[1], srcSize[0], srcSize[1],
		rgb[0], rgb[1], rgb[2], alpha, drawflag,
	)
}
