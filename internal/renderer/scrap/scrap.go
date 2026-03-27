package scrap

// ScrapAllocator implements skyline bin-packing for rectangular regions
// within a fixed-size atlas page. This is the classic Quake scrap texture
// approach: track per-column allocation height and find the best-fit
// position for each new rectangle.
type ScrapAllocator struct {
	width   int   // Atlas page width.
	height  int   // Atlas page height.
	skyline []int // Per-column allocation height (len == width).
}

// NewScrapAllocator creates a skyline allocator for a fixed-size atlas page.
func NewScrapAllocator(width, height int) *ScrapAllocator {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}

	return &ScrapAllocator{
		width:   width,
		height:  height,
		skyline: make([]int, width),
	}
}

// Alloc places a w×h rectangle using skyline best-fit placement.
//
// The skyline stores, for each x column, the next free y position.
// For every valid x start, we compute the maximum skyline value across the
// requested width and choose the position with the smallest max value. This
// mirrors classic Quake scrap packing behavior.
func (a *ScrapAllocator) Alloc(w, h int) (x, y int, ok bool) {
	if a == nil || w <= 0 || h <= 0 || w > a.width || h > a.height {
		return 0, 0, false
	}

	bestX, bestY := -1, a.height

	for tryX := 0; tryX <= a.width-w; tryX++ {
		maxY := 0
		for col := tryX; col < tryX+w; col++ {
			if a.skyline[col] > maxY {
				maxY = a.skyline[col]
			}
		}

		if maxY+h <= a.height && maxY < bestY {
			bestX = tryX
			bestY = maxY
		}
	}

	if bestX < 0 {
		return 0, 0, false
	}

	newHeight := bestY + h
	for col := bestX; col < bestX+w; col++ {
		a.skyline[col] = newHeight
	}

	return bestX, bestY, true
}

// Reset clears all allocations in the atlas page.
func (a *ScrapAllocator) Reset() {
	if a == nil {
		return
	}
	for i := range a.skyline {
		a.skyline[i] = 0
	}
}

// UsedHeight returns the tallest occupied skyline column.
func (a *ScrapAllocator) UsedHeight() int {
	if a == nil {
		return 0
	}
	maxH := 0
	for _, h := range a.skyline {
		if h > maxH {
			maxH = h
		}
	}
	return maxH
}

// FreeArea returns an approximate free area based on skyline occupancy.
//
// Because skyline tracks column heights (not exact holes), this value is an
// approximation for fragmented atlases.
func (a *ScrapAllocator) FreeArea() int {
	if a == nil {
		return 0
	}
	total := a.width * a.height
	used := 0
	for _, h := range a.skyline {
		used += h
	}
	free := total - used
	if free < 0 {
		return 0
	}
	return free
}
