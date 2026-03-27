package scrap

import "fmt"

const (
	defaultScrapMaxItemW = 128
	defaultScrapMaxItemH = 128
	scrapBorderPixels    = 1
)

// ScrapUVRect holds normalized UV coordinates for a scrap entry
// within its atlas page.
type ScrapUVRect struct {
	U0, V0 float32 // Top-left UV.
	U1, V1 float32 // Bottom-right UV.
}

// ScrapEntry represents an allocated region in the scrap atlas.
type ScrapEntry struct {
	PageIndex int         // Which atlas page this entry lives in.
	X, Y      int         // Pixel position within the page.
	Width     int         // Entry width in pixels.
	Height    int         // Entry height in pixels.
	UV        ScrapUVRect // Normalized UV coordinates.
}

// ScrapPage represents a single atlas page with its allocator
// and CPU-side pixel buffer.
type ScrapPage struct {
	Allocator *ScrapAllocator
	Pixels    []byte // RGBA pixel data (pageWidth * pageHeight * 4).
	Dirty     bool   // Needs GPU re-upload.
	Width     int
	Height    int
}

// ScrapAtlas manages one or more atlas pages for packing small textures.
//
// This is a CPU-side manager only: it tracks where sub-images live,
// stores their uploaded RGBA data, and marks pages dirty for deferred GPU
// uploads performed by backend-specific code.
type ScrapAtlas struct {
	pages      []*ScrapPage
	pageWidth  int
	pageHeight int
	maxItemW   int // Maximum item width (default 128).
	maxItemH   int // Maximum item height (default 128).
}

// NewScrapAtlas creates an atlas with one initial empty page.
func NewScrapAtlas(pageWidth, pageHeight int) *ScrapAtlas {
	if pageWidth < 0 {
		pageWidth = 0
	}
	if pageHeight < 0 {
		pageHeight = 0
	}

	a := &ScrapAtlas{
		pageWidth:  pageWidth,
		pageHeight: pageHeight,
		maxItemW:   defaultScrapMaxItemW,
		maxItemH:   defaultScrapMaxItemH,
	}
	a.addPage()
	return a
}

// SetMaxItemSize sets the maximum per-entry dimensions allowed for allocation.
func (a *ScrapAtlas) SetMaxItemSize(w, h int) {
	if a == nil {
		return
	}
	if w > 0 {
		a.maxItemW = w
	}
	if h > 0 {
		a.maxItemH = h
	}
}

// Alloc reserves a w×h logical region in one of the atlas pages.
//
// Allocation is first-fit across existing pages. If all existing pages are
// full, a new page is created and allocation is retried there. Each entry
// reserves a 1-pixel border on all sides so uploads can replicate edge texels
// and match Quake's scrap clamping behavior without changing the returned UVs.
func (a *ScrapAtlas) Alloc(w, h int) (*ScrapEntry, error) {
	if a == nil {
		return nil, fmt.Errorf("scrap atlas is nil")
	}
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid scrap size %dx%d", w, h)
	}
	if w > a.maxItemW || h > a.maxItemH {
		return nil, fmt.Errorf("scrap item too large %dx%d, max is %dx%d", w, h, a.maxItemW, a.maxItemH)
	}
	paddedW := w + scrapBorderPixels*2
	paddedH := h + scrapBorderPixels*2
	if paddedW > a.pageWidth || paddedH > a.pageHeight {
		return nil, fmt.Errorf("scrap item %dx%d exceeds page size %dx%d", w, h, a.pageWidth, a.pageHeight)
	}

	for pageIndex, page := range a.pages {
		x, y, ok := page.Allocator.Alloc(paddedW, paddedH)
		if ok {
			return &ScrapEntry{
				PageIndex: pageIndex,
				X:         x + scrapBorderPixels,
				Y:         y + scrapBorderPixels,
				Width:     w,
				Height:    h,
				UV:        makeScrapUVRect(x+scrapBorderPixels, y+scrapBorderPixels, w, h, a.pageWidth, a.pageHeight),
			}, nil
		}
	}

	page := a.addPage()
	x, y, ok := page.Allocator.Alloc(paddedW, paddedH)
	if !ok {
		return nil, fmt.Errorf("alloc %dx%d failed in new scrap page %dx%d", w, h, a.pageWidth, a.pageHeight)
	}

	return &ScrapEntry{
		PageIndex: len(a.pages) - 1,
		X:         x + scrapBorderPixels,
		Y:         y + scrapBorderPixels,
		Width:     w,
		Height:    h,
		UV:        makeScrapUVRect(x+scrapBorderPixels, y+scrapBorderPixels, w, h, a.pageWidth, a.pageHeight),
	}, nil
}

// Upload copies RGBA pixels into the allocated entry region and marks the page dirty.
// Invalid input is ignored to keep callers simple and avoid panics.
func (a *ScrapAtlas) Upload(entry *ScrapEntry, rgba []byte) {
	if a == nil || entry == nil {
		return
	}
	if entry.PageIndex < 0 || entry.PageIndex >= len(a.pages) {
		return
	}
	if entry.Width <= 0 || entry.Height <= 0 {
		return
	}
	need := entry.Width * entry.Height * 4
	if len(rgba) < need {
		return
	}

	page := a.pages[entry.PageIndex]
	if entry.X-scrapBorderPixels < 0 || entry.Y-scrapBorderPixels < 0 || entry.X+entry.Width+scrapBorderPixels > page.Width || entry.Y+entry.Height+scrapBorderPixels > page.Height {
		return
	}

	stride := page.Width * 4
	for row := -scrapBorderPixels; row < entry.Height+scrapBorderPixels; row++ {
		srcRow := row
		if srcRow < 0 {
			srcRow = 0
		} else if srcRow >= entry.Height {
			srcRow = entry.Height - 1
		}

		srcStart := srcRow * entry.Width * 4
		srcRowPixels := rgba[srcStart : srcStart+entry.Width*4]
		dstStart := (entry.Y+row)*stride + (entry.X-scrapBorderPixels)*4

		copy(page.Pixels[dstStart:dstStart+4], srcRowPixels[:4])
		copy(page.Pixels[dstStart+4:dstStart+4+entry.Width*4], srcRowPixels)
		copy(page.Pixels[dstStart+4+entry.Width*4:dstStart+8+entry.Width*4], srcRowPixels[len(srcRowPixels)-4:])
	}
	page.Dirty = true
}

// PageCount returns the number of atlas pages.
func (a *ScrapAtlas) PageCount() int {
	if a == nil {
		return 0
	}
	return len(a.pages)
}

// Page returns the page at index, or nil when out of range.
func (a *ScrapAtlas) Page(index int) *ScrapPage {
	if a == nil || index < 0 || index >= len(a.pages) {
		return nil
	}
	return a.pages[index]
}

// DirtyPages returns indices of pages that require GPU upload.
func (a *ScrapAtlas) DirtyPages() []int {
	if a == nil {
		return nil
	}

	dirty := make([]int, 0, len(a.pages))
	for i, page := range a.pages {
		if page.Dirty {
			dirty = append(dirty, i)
		}
	}
	return dirty
}

// ClearDirty clears the dirty flag on a page.
func (a *ScrapAtlas) ClearDirty(pageIndex int) {
	if a == nil || pageIndex < 0 || pageIndex >= len(a.pages) {
		return
	}
	a.pages[pageIndex].Dirty = false
}

// Reset clears all page allocations and uploaded data.
func (a *ScrapAtlas) Reset() {
	if a == nil {
		return
	}
	a.pages = nil
	a.addPage()
}

func (a *ScrapAtlas) addPage() *ScrapPage {
	page := &ScrapPage{
		Allocator: NewScrapAllocator(a.pageWidth, a.pageHeight),
		Pixels:    make([]byte, a.pageWidth*a.pageHeight*4),
		Width:     a.pageWidth,
		Height:    a.pageHeight,
	}
	a.pages = append(a.pages, page)
	return page
}

func makeScrapUVRect(x, y, w, h, pageWidth, pageHeight int) ScrapUVRect {
	return ScrapUVRect{
		U0: float32(x) / float32(pageWidth),
		V0: float32(y) / float32(pageHeight),
		U1: float32(x+w) / float32(pageWidth),
		V1: float32(y+h) / float32(pageHeight),
	}
}
