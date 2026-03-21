package renderer

import "github.com/ironwail/ironwail-go/internal/model"

func resolveAliasSkinSlot(hdr *model.AliasHeader, skinNum int, timeSeconds float64, available int) int {
	if available <= 0 {
		return -1
	}
	if hdr == nil {
		if skinNum < 0 {
			skinNum = 0
		}
		return skinNum % available
	}
	index := hdr.ResolveSkinFrame(skinNum, timeSeconds)
	if index < 0 {
		return 0
	}
	if index >= available {
		return available - 1
	}
	return index
}
