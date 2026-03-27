package gogpu

import (
	ginput "github.com/gogpu/gogpu/input"
	"github.com/gogpu/gpucontext"
	iinput "github.com/ironwail/ironwail-go/internal/input"
)

type PollingKeyPair struct {
	Src ginput.Key
	Dst int
}

var PollingKeyMap = func() []PollingKeyPair {
	pairs := []PollingKeyPair{
		{Src: ginput.KeyEscape, Dst: iinput.KEscape},
		{Src: ginput.KeyEnter, Dst: iinput.KEnter},
		{Src: ginput.KeyNumpadEnter, Dst: iinput.KEnter},
		{Src: ginput.KeyTab, Dst: iinput.KTab},
		{Src: ginput.KeySpace, Dst: iinput.KSpace},
		{Src: ginput.KeyBackspace, Dst: iinput.KBackspace},
		{Src: ginput.KeyUp, Dst: iinput.KUpArrow},
		{Src: ginput.KeyDown, Dst: iinput.KDownArrow},
		{Src: ginput.KeyLeft, Dst: iinput.KLeftArrow},
		{Src: ginput.KeyRight, Dst: iinput.KRightArrow},
		{Src: ginput.KeyShiftLeft, Dst: iinput.KShift},
		{Src: ginput.KeyShiftRight, Dst: iinput.KShift},
		{Src: ginput.KeyControlLeft, Dst: iinput.KCtrl},
		{Src: ginput.KeyControlRight, Dst: iinput.KCtrl},
		{Src: ginput.KeyAltLeft, Dst: iinput.KAlt},
		{Src: ginput.KeyAltRight, Dst: iinput.KAlt},
	}

	letterKeys := []ginput.Key{
		ginput.KeyA, ginput.KeyB, ginput.KeyC, ginput.KeyD, ginput.KeyE, ginput.KeyF,
		ginput.KeyG, ginput.KeyH, ginput.KeyI, ginput.KeyJ, ginput.KeyK, ginput.KeyL,
		ginput.KeyM, ginput.KeyN, ginput.KeyO, ginput.KeyP, ginput.KeyQ, ginput.KeyR,
		ginput.KeyS, ginput.KeyT, ginput.KeyU, ginput.KeyV, ginput.KeyW, ginput.KeyX,
		ginput.KeyY, ginput.KeyZ,
	}
	for index, key := range letterKeys {
		pairs = append(pairs, PollingKeyPair{Src: key, Dst: int('a' + index)})
	}

	numberKeys := []ginput.Key{
		ginput.Key0, ginput.Key1, ginput.Key2, ginput.Key3, ginput.Key4,
		ginput.Key5, ginput.Key6, ginput.Key7, ginput.Key8, ginput.Key9,
	}
	for index, key := range numberKeys {
		pairs = append(pairs, PollingKeyPair{Src: key, Dst: int('0' + index)})
	}

	return pairs
}()

// MapGPUContextMouseButton maps a gpucontext mouse button to a Quake engine key code.
func MapGPUContextMouseButton(button gpucontext.MouseButton) int {
	switch button {
	case gpucontext.MouseButtonLeft:
		return iinput.KMouse1
	case gpucontext.MouseButtonRight:
		return iinput.KMouse2
	case gpucontext.MouseButtonMiddle:
		return iinput.KMouse3
	case gpucontext.MouseButton4:
		return iinput.KMouse4
	case gpucontext.MouseButton5:
		return iinput.KMouse5
	default:
		return -1
	}
}

// MapGPUContextKey maps a gpucontext key to a Quake engine key code.
func MapGPUContextKey(key gpucontext.Key) int {
	switch key {
	case gpucontext.KeyEscape:
		return iinput.KEscape
	case gpucontext.KeyEnter, gpucontext.KeyNumpadEnter:
		return iinput.KEnter
	case gpucontext.KeyTab:
		return iinput.KTab
	case gpucontext.KeySpace:
		return iinput.KSpace
	case gpucontext.KeyBackspace:
		return iinput.KBackspace
	case gpucontext.KeyUp:
		return iinput.KUpArrow
	case gpucontext.KeyDown:
		return iinput.KDownArrow
	case gpucontext.KeyLeft:
		return iinput.KLeftArrow
	case gpucontext.KeyRight:
		return iinput.KRightArrow
	case gpucontext.KeyLeftShift, gpucontext.KeyRightShift:
		return iinput.KShift
	case gpucontext.KeyLeftControl, gpucontext.KeyRightControl:
		return iinput.KCtrl
	case gpucontext.KeyLeftAlt, gpucontext.KeyRightAlt:
		return iinput.KAlt
	}

	if key >= gpucontext.KeyA && key <= gpucontext.KeyZ {
		return int('a' + (key - gpucontext.KeyA))
	}
	if key >= gpucontext.Key0 && key <= gpucontext.Key9 {
		return int('0' + (key - gpucontext.Key0))
	}

	return -1
}
