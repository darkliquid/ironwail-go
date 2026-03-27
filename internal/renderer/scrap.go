package renderer

import scrapimpl "github.com/ironwail/ironwail-go/internal/renderer/scrap"

type ScrapAllocator = scrapimpl.ScrapAllocator

func NewScrapAllocator(width, height int) *ScrapAllocator {
	return scrapimpl.NewScrapAllocator(width, height)
}
