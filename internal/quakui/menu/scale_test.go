package menu

import (
	"testing"

	"github.com/gogpu/ui/geometry"
)

func TestComputeMenuTransform_320x200(t *testing.T) {
	tf := ComputeMenuTransform(MenuScaleParams{
		WindowWidth:  320,
		WindowHeight: 200,
	})
	if tf.Scale != 1.0 {
		t.Fatalf("Scale = %f, want 1.0", tf.Scale)
	}
	if tf.OriginX != 0 || tf.OriginY != 0 {
		t.Fatalf("Origin = (%f, %f), want (0, 0)", tf.OriginX, tf.OriginY)
	}
}

func TestComputeMenuTransform_1280x720(t *testing.T) {
	tf := ComputeMenuTransform(MenuScaleParams{
		WindowWidth:  1280,
		WindowHeight: 720,
	})
	// originX = (1280 - 320)/2 = 480
	// originY = (720 - 200)/2 = 260
	if tf.OriginX != 480 || tf.OriginY != 260 {
		t.Fatalf("Origin = (%f, %f), want (480, 260)", tf.OriginX, tf.OriginY)
	}
}

func TestComputeMenuTransform_1892x1072(t *testing.T) {
	tf := ComputeMenuTransform(MenuScaleParams{
		WindowWidth:  1892,
		WindowHeight: 1072,
	})
	// originX = (1892 - 320)/2 = 786
	// originY = (1072 - 200)/2 = 436
	if tf.OriginX != 786 || tf.OriginY != 436 {
		t.Fatalf("Origin = (%f, %f), want (786, 436)", tf.OriginX, tf.OriginY)
	}
}

func TestComputeMenuTransform_CustomScale(t *testing.T) {
	tf := ComputeMenuTransform(MenuScaleParams{
		WindowWidth:  1280,
		WindowHeight: 720,
		MenuScale:    2.0,
	})
	if tf.Scale != 2.0 {
		t.Fatalf("Scale = %f, want 2.0", tf.Scale)
	}
	// originX = (1280 - 320*2)/2 = 320
	// originY = (720 - 200*2)/2 = 160
	if tf.OriginX != 320 || tf.OriginY != 160 {
		t.Fatalf("Origin = (%f, %f), want (320, 160)", tf.OriginX, tf.OriginY)
	}
}

func TestMenuTransform_PointMapping(t *testing.T) {
	tf := MenuTransform{
		Scale:   2.0,
		OriginX: 100,
		OriginY: 50,
	}
	pt := geometry.Pt(10, 20)
	mapped := tf.TransformPoint(pt)
	if mapped.X != 120 || mapped.Y != 90 {
		t.Fatalf("TransformPoint = (%f, %f), want (120, 90)", mapped.X, mapped.Y)
	}
	unmapped := tf.InverseTransformPoint(mapped)
	if unmapped.X != 10 || unmapped.Y != 20 {
		t.Fatalf("InverseTransformPoint = (%f, %f), want (10, 20)", unmapped.X, unmapped.Y)
	}
}
