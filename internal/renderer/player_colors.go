package renderer

// TranslatePlayerSkinPixels remaps Quake player shirt and pants ranges to the
// selected top/bottom colors while leaving all other palette indices untouched.
func TranslatePlayerSkinPixels(pixels []byte, topColor, bottomColor int) []byte {
	if len(pixels) == 0 {
		return nil
	}

	translated := make([]byte, len(pixels))
	copy(translated, pixels)

	topStart := byte((topColor & 15) << 4)
	bottomStart := byte((bottomColor & 15) << 4)

	for i, pixel := range translated {
		switch {
		case pixel >= 16 && pixel < 32:
			translated[i] = translatedPlayerColor(topStart, pixel-16)
		case pixel >= 96 && pixel < 112:
			translated[i] = translatedPlayerColor(bottomStart, pixel-96)
		}
	}

	return translated
}

func translatedPlayerColor(start, offset byte) byte {
	if start < 128 {
		return start + offset
	}
	return start + (15 - offset)
}

func splitPlayerColors(packed byte) (topColor, bottomColor int) {
	return int((packed >> 4) & 15), int(packed & 15)
}
