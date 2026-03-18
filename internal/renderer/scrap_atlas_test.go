package renderer

import (
	"math"
	"reflect"
	"testing"
)

func TestScrapAtlasAllocBasicAndUV(t *testing.T) {
	atlas := NewScrapAtlas(16, 16)

	entry, err := atlas.Alloc(8, 4)
	if err != nil {
		t.Fatalf("Alloc(8,4) returned error: %v", err)
	}
	if entry.PageIndex != 0 || entry.X != 0 || entry.Y != 0 || entry.Width != 8 || entry.Height != 4 {
		t.Fatalf("entry = %+v, want page=0 pos=(0,0) size=8x4", entry)
	}

	assertFloat32Equal(t, entry.UV.U0, 0.0)
	assertFloat32Equal(t, entry.UV.V0, 0.0)
	assertFloat32Equal(t, entry.UV.U1, 8.0/16.0)
	assertFloat32Equal(t, entry.UV.V1, 4.0/16.0)
}

func TestScrapAtlasUploadCopiesPixels(t *testing.T) {
	atlas := NewScrapAtlas(4, 4)

	entry, err := atlas.Alloc(2, 2)
	if err != nil {
		t.Fatalf("Alloc(2,2) error: %v", err)
	}

	rgba := []byte{
		1, 2, 3, 4,
		5, 6, 7, 8,
		9, 10, 11, 12,
		13, 14, 15, 16,
	}
	atlas.Upload(entry, rgba)

	page := atlas.Page(0)
	if page == nil {
		t.Fatal("expected page 0")
	}
	if !page.Dirty {
		t.Fatal("expected page 0 to be dirty after upload")
	}

	stride := page.Width * 4
	row0Start := entry.Y*stride + entry.X*4
	row1Start := (entry.Y+1)*stride + entry.X*4

	if got := page.Pixels[row0Start : row0Start+8]; !reflect.DeepEqual(got, rgba[0:8]) {
		t.Fatalf("row0 pixels = %v, want %v", got, rgba[0:8])
	}
	if got := page.Pixels[row1Start : row1Start+8]; !reflect.DeepEqual(got, rgba[8:16]) {
		t.Fatalf("row1 pixels = %v, want %v", got, rgba[8:16])
	}
}

func TestScrapAtlasAutoPageGrowth(t *testing.T) {
	atlas := NewScrapAtlas(4, 4)

	first, err := atlas.Alloc(4, 4)
	if err != nil {
		t.Fatalf("first alloc error: %v", err)
	}
	second, err := atlas.Alloc(4, 4)
	if err != nil {
		t.Fatalf("second alloc error: %v", err)
	}

	if first.PageIndex != 0 {
		t.Fatalf("first page = %d, want 0", first.PageIndex)
	}
	if second.PageIndex != 1 {
		t.Fatalf("second page = %d, want 1", second.PageIndex)
	}
	if atlas.PageCount() != 2 {
		t.Fatalf("PageCount() = %d, want 2", atlas.PageCount())
	}
}

func TestScrapAtlasMultipleAllocationsAcrossPages(t *testing.T) {
	atlas := NewScrapAtlas(4, 4)

	entries := make([]*ScrapEntry, 0, 5)
	for i := 0; i < 5; i++ {
		e, err := atlas.Alloc(2, 2)
		if err != nil {
			t.Fatalf("alloc %d error: %v", i, err)
		}
		entries = append(entries, e)
	}

	for i := 0; i < 4; i++ {
		if entries[i].PageIndex != 0 {
			t.Fatalf("entry %d page = %d, want 0", i, entries[i].PageIndex)
		}
	}
	if entries[4].PageIndex != 1 {
		t.Fatalf("fifth entry page = %d, want 1", entries[4].PageIndex)
	}
}

func TestScrapAtlasMaxItemSizeRejection(t *testing.T) {
	atlas := NewScrapAtlas(256, 1024)
	atlas.SetMaxItemSize(8, 8)

	if _, err := atlas.Alloc(9, 8); err == nil {
		t.Fatal("expected width limit error")
	}
	if _, err := atlas.Alloc(8, 9); err == nil {
		t.Fatal("expected height limit error")
	}
	if _, err := atlas.Alloc(8, 8); err != nil {
		t.Fatalf("Alloc(8,8) unexpected error: %v", err)
	}
}

func TestScrapAtlasDirtyPagesAndClearDirty(t *testing.T) {
	atlas := NewScrapAtlas(4, 4)

	e1, err := atlas.Alloc(4, 4)
	if err != nil {
		t.Fatalf("alloc page 0 error: %v", err)
	}
	e2, err := atlas.Alloc(4, 4)
	if err != nil {
		t.Fatalf("alloc page 1 error: %v", err)
	}

	atlas.Upload(e1, make([]byte, e1.Width*e1.Height*4))
	atlas.Upload(e2, make([]byte, e2.Width*e2.Height*4))

	if got, want := atlas.DirtyPages(), []int{0, 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("DirtyPages() = %v, want %v", got, want)
	}

	atlas.ClearDirty(0)
	if got, want := atlas.DirtyPages(), []int{1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("DirtyPages() after ClearDirty(0) = %v, want %v", got, want)
	}

	atlas.ClearDirty(1)
	if got := atlas.DirtyPages(); len(got) != 0 {
		t.Fatalf("DirtyPages() after clear all = %v, want []", got)
	}
}

func TestScrapAtlasPageAccessAndReset(t *testing.T) {
	atlas := NewScrapAtlas(8, 8)

	entry, err := atlas.Alloc(2, 2)
	if err != nil {
		t.Fatalf("alloc error: %v", err)
	}
	atlas.Upload(entry, make([]byte, 2*2*4))

	if atlas.Page(-1) != nil {
		t.Fatal("Page(-1) should be nil")
	}
	if atlas.Page(999) != nil {
		t.Fatal("Page(999) should be nil")
	}
	if atlas.Page(0) == nil {
		t.Fatal("Page(0) should exist")
	}

	atlas.Reset()

	if atlas.PageCount() != 1 {
		t.Fatalf("PageCount() after reset = %d, want 1", atlas.PageCount())
	}
	if got := atlas.DirtyPages(); len(got) != 0 {
		t.Fatalf("DirtyPages() after reset = %v, want []", got)
	}
	page := atlas.Page(0)
	if page == nil {
		t.Fatal("expected page 0 after reset")
	}
	if page.Dirty {
		t.Fatal("page should not be dirty after reset")
	}
	for i, b := range page.Pixels {
		if b != 0 {
			t.Fatalf("pixel %d = %d, want 0", i, b)
		}
	}
}

func TestScrapAtlasUVNormalization(t *testing.T) {
	atlas := NewScrapAtlas(16, 32)

	entry, err := atlas.Alloc(4, 8)
	if err != nil {
		t.Fatalf("alloc error: %v", err)
	}
	if entry.X != 0 || entry.Y != 0 {
		t.Fatalf("expected first alloc at origin, got (%d,%d)", entry.X, entry.Y)
	}

	assertFloat32Equal(t, entry.UV.U0, float32(entry.X)/16.0)
	assertFloat32Equal(t, entry.UV.V0, float32(entry.Y)/32.0)
	assertFloat32Equal(t, entry.UV.U1, float32(entry.X+entry.Width)/16.0)
	assertFloat32Equal(t, entry.UV.V1, float32(entry.Y+entry.Height)/32.0)
}

func assertFloat32Equal(t *testing.T, got, want float32) {
	t.Helper()
	if math.Abs(float64(got-want)) > 1e-6 {
		t.Fatalf("float mismatch: got %f want %f", got, want)
	}
}
