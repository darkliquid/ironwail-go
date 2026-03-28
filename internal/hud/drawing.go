// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package hud

import (
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

// DrawNumber renders a numeric value at the specified position using character glyphs.
// It draws right-aligned numbers with optional minimum digit count.
// x, y are in screen pixel coordinates.
func DrawNumber(rc renderer.RenderContext, x, y int, num int, digits int) {
	if rc == nil {
		return
	}

	// Handle negative numbers
	isNegative := num < 0
	if isNegative {
		num = -num
	}

	// Convert number to string for digit extraction
	numStr := ""
	if num == 0 {
		numStr = "0"
	} else {
		for n := num; n > 0; n /= 10 {
			numStr = string(rune('0'+(n%10))) + numStr
		}
	}

	// Pad with leading spaces if needed
	for len(numStr) < digits {
		numStr = " " + numStr
	}

	// Add negative sign if needed
	if isNegative {
		numStr = "-" + numStr
	}

	// Draw each character (right-aligned from x)
	charWidth := 8
	startX := x - (len(numStr) * charWidth)
	for i, ch := range numStr {
		if ch != ' ' {
			rc.DrawCharacter(startX+i*charWidth, y, int(ch))
		}
	}
}

// DrawString renders a text string at the specified position using character glyphs.
// x, y are in screen pixel coordinates.
func DrawString(rc renderer.RenderContext, x, y int, text string) {
	if rc == nil {
		return
	}

	charWidth := 8
	for i, ch := range text {
		rc.DrawCharacter(x+i*charWidth, y, int(ch))
	}
}
