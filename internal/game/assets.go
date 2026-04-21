package game

// QueueRendererAssets queues palette and conchars data to be applied to the renderer.
func (g *Game) QueueRendererAssets(palette []byte, conchars []byte) {
	if g.pendingRendererAssets == nil {
		return
	}
	g.pendingRendererAssets.mu.Lock()
	defer g.pendingRendererAssets.mu.Unlock()

	g.pendingRendererAssets.Palette = append(g.pendingRendererAssets.Palette[:0], palette...)
	g.pendingRendererAssets.Conchars = append(g.pendingRendererAssets.Conchars[:0], conchars...)
	g.pendingRendererAssets.HasPending = true
}

// QueueRendererWorldClear queues a world clear operation.
func (g *Game) QueueRendererWorldClear() {
	if g.pendingRendererAssets == nil {
		return
	}
	g.pendingRendererAssets.mu.Lock()
	defer g.pendingRendererAssets.mu.Unlock()

	g.pendingRendererAssets.ClearWorld = true
}

// ApplyQueuedRendererAssets applies any queued renderer asset updates.
func (g *Game) ApplyQueuedRendererAssets() {
	if g.Renderer == nil || g.pendingRendererAssets == nil {
		return
	}

	g.pendingRendererAssets.mu.Lock()
	if !g.pendingRendererAssets.HasPending && !g.pendingRendererAssets.ClearWorld {
		g.pendingRendererAssets.mu.Unlock()
		return
	}
	clearWorld := g.pendingRendererAssets.ClearWorld
	palette := append([]byte(nil), g.pendingRendererAssets.Palette...)
	conchars := append([]byte(nil), g.pendingRendererAssets.Conchars...)
	g.pendingRendererAssets.Palette = g.pendingRendererAssets.Palette[:0]
	g.pendingRendererAssets.Conchars = g.pendingRendererAssets.Conchars[:0]
	g.pendingRendererAssets.HasPending = false
	g.pendingRendererAssets.ClearWorld = false
	g.pendingRendererAssets.mu.Unlock()

	if clearWorld {
		if clearer, ok := any(g.Renderer).(interface{ ClearWorld() }); ok {
			clearer.ClearWorld()
		}
	}
	if len(palette) >= 768 {
		g.Renderer.SetPalette(palette)
	}
	if len(conchars) >= 128*128 {
		g.Renderer.SetConchars(conchars)
	}
}
