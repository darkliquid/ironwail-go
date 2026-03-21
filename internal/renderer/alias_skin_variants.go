package renderer

func aliasSkinVariantRGBA(pixels []byte, palette []byte, colorMap uint32, isPlayer bool) (baseRGBA, fullbrightRGBA []byte) {
	source := pixels
	if isPlayer {
		topColor, bottomColor := splitPlayerColors(byte(colorMap))
		source = TranslatePlayerSkinPixels(pixels, topColor, bottomColor)
	}
	fullbrightRGBA, _ = ConvertPaletteToFullbrightRGBA(source, palette)
	return ConvertPaletteToRGBA(source, palette), fullbrightRGBA
}
