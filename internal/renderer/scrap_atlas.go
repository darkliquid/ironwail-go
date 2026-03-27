package renderer

import scrapimpl "github.com/ironwail/ironwail-go/internal/renderer/scrap"

type ScrapUVRect = scrapimpl.ScrapUVRect
type ScrapEntry = scrapimpl.ScrapEntry
type ScrapPage = scrapimpl.ScrapPage
type ScrapAtlas = scrapimpl.ScrapAtlas

func NewScrapAtlas(pageWidth, pageHeight int) *ScrapAtlas {
	return scrapimpl.NewScrapAtlas(pageWidth, pageHeight)
}
