//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"testing"

	"github.com/gogpu/gogpu/gmath"
)

func TestProjectWorldPointToScreenCenter(t *testing.T) {
	vp := gmath.Identity4()

	x, y, ok := projectWorldPointToScreen([3]float32{0, 0, 0}, vp, 801, 601)
	if !ok {
		t.Fatal("projectWorldPointToScreen returned not visible for center point")
	}
	if x != 400 || y != 300 {
		t.Fatalf("projectWorldPointToScreen center = (%d,%d), want (400,300)", x, y)
	}
}

func TestProjectWorldPointToScreenRejectsOutOfClip(t *testing.T) {
	vp := gmath.Identity4()

	if _, _, ok := projectWorldPointToScreen([3]float32{2, 0, 0}, vp, 800, 600); ok {
		t.Fatal("projectWorldPointToScreen accepted point outside clip space")
	}
}

func TestProjectWorldPointToScreenRejectsNonPositiveW(t *testing.T) {
	var vp gmath.Mat4 = gmath.Identity4()
	vp[3] = 0
	vp[7] = 0
	vp[11] = -1
	vp[15] = 0

	if _, _, ok := projectWorldPointToScreen([3]float32{0, 0, 1}, vp, 800, 600); ok {
		t.Fatal("projectWorldPointToScreen accepted point with non-positive clip W")
	}
}
