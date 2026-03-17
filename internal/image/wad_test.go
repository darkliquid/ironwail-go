package image

import "testing"

func TestSubPic(t *testing.T) {
	// 4x4 image with sequential pixel values.
	src := &QPic{
		Width:  4,
		Height: 4,
		Pixels: []byte{
			0, 1, 2, 3,
			4, 5, 6, 7,
			8, 9, 10, 11,
			12, 13, 14, 15,
		},
	}

	sub := src.SubPic(1, 1, 2, 2)
	if sub.Width != 2 || sub.Height != 2 {
		t.Fatalf("expected 2x2, got %dx%d", sub.Width, sub.Height)
	}
	want := []byte{5, 6, 9, 10}
	for i, v := range want {
		if sub.Pixels[i] != v {
			t.Fatalf("pixel %d: expected %d, got %d", i, v, sub.Pixels[i])
		}
	}
}

func TestSubPicClamp(t *testing.T) {
	src := &QPic{
		Width:  4,
		Height: 4,
		Pixels: make([]byte, 16),
	}

	// Request extends past bounds — should clamp.
	sub := src.SubPic(3, 3, 5, 5)
	if sub.Width != 1 || sub.Height != 1 {
		t.Fatalf("expected 1x1 after clamp, got %dx%d", sub.Width, sub.Height)
	}
}

func TestSubPicNegativeOrigin(t *testing.T) {
	src := &QPic{
		Width:  4,
		Height: 4,
		Pixels: make([]byte, 16),
	}

	sub := src.SubPic(-1, -1, 2, 2)
	if sub.Width != 2 || sub.Height != 2 {
		t.Fatalf("expected 2x2 after negative clamp, got %dx%d", sub.Width, sub.Height)
	}
}

func TestSubPicEmpty(t *testing.T) {
	src := &QPic{
		Width:  4,
		Height: 4,
		Pixels: make([]byte, 16),
	}

	sub := src.SubPic(4, 4, 2, 2)
	if sub.Width != 0 || sub.Height != 0 {
		t.Fatalf("expected 0x0 for out-of-bounds, got %dx%d", sub.Width, sub.Height)
	}
}
