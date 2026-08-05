package overlay

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/image"
)

func TestCompositor2D(t *testing.T) {
	c := NewCompositor2D(320, 200)
	if c.IsDirty() {
		t.Fatal("expected fresh compositor to be clean")
	}

	c.DrawString(10, 10, "Hello")
	if !c.IsDirty() {
		t.Fatal("expected compositor to be dirty after DrawString")
	}

	pic := &image.QPic{Width: 16, Height: 16}
	c.DrawPic(0, 0, pic)
	c.DrawCharacter(5, 5, 65)
}
