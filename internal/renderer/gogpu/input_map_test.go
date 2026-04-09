package gogpu

import (
	"testing"

	iinput "github.com/darkliquid/ironwail-go/internal/input"
	ginput "github.com/gogpu/gogpu/input"
	"github.com/gogpu/gpucontext"
)

func TestMapGPUContextKeyMapsQuakeCriticalKeys(t *testing.T) {
	tests := []struct {
		name string
		key  gpucontext.Key
		want int
	}{
		{name: "grave", key: gpucontext.KeyGrave, want: int('`')},
		{name: "apostrophe", key: gpucontext.KeyApostrophe, want: int('\'')},
		{name: "comma", key: gpucontext.KeyComma, want: int(',')},
		{name: "minus", key: gpucontext.KeyMinus, want: int('-')},
		{name: "period", key: gpucontext.KeyPeriod, want: int('.')},
		{name: "slash", key: gpucontext.KeySlash, want: int('/')},
		{name: "semicolon", key: gpucontext.KeySemicolon, want: int(';')},
		{name: "equal", key: gpucontext.KeyEqual, want: int('=')},
		{name: "left bracket", key: gpucontext.KeyLeftBracket, want: int('[')},
		{name: "backslash", key: gpucontext.KeyBackslash, want: int('\\')},
		{name: "right bracket", key: gpucontext.KeyRightBracket, want: int(']')},
		{name: "scroll lock", key: gpucontext.KeyScrollLock, want: iinput.KScrollLock},
		{name: "print screen", key: gpucontext.KeyPrintScreen, want: iinput.KPrintScreen},
		{name: "pause", key: gpucontext.KeyPause, want: iinput.KPause},
		{name: "insert", key: gpucontext.KeyInsert, want: iinput.KIns},
		{name: "delete", key: gpucontext.KeyDelete, want: iinput.KDel},
		{name: "home", key: gpucontext.KeyHome, want: iinput.KHome},
		{name: "end", key: gpucontext.KeyEnd, want: iinput.KEnd},
		{name: "page up", key: gpucontext.KeyPageUp, want: iinput.KPgUp},
		{name: "page down", key: gpucontext.KeyPageDown, want: iinput.KPgDn},
		{name: "left super", key: gpucontext.KeyLeftSuper, want: iinput.KCommand},
		{name: "f1", key: gpucontext.KeyF1, want: iinput.KF1},
		{name: "f12", key: gpucontext.KeyF12, want: iinput.KF12},
		{name: "numpad 0", key: gpucontext.KeyNumpad0, want: iinput.KKpIns},
		{name: "numpad 9", key: gpucontext.KeyNumpad9, want: iinput.KKpPgUp},
		{name: "numpad divide", key: gpucontext.KeyNumpadDivide, want: iinput.KKpSlash},
		{name: "numpad multiply", key: gpucontext.KeyNumpadMultiply, want: iinput.KKpStar},
		{name: "numpad subtract", key: gpucontext.KeyNumpadSubtract, want: iinput.KKpMinus},
		{name: "numpad add", key: gpucontext.KeyNumpadAdd, want: iinput.KKpPlus},
		{name: "numpad decimal", key: gpucontext.KeyNumpadDecimal, want: iinput.KKpDel},
		{name: "num lock", key: gpucontext.KeyNumLock, want: iinput.KKpNumLock},
	}

	for _, tc := range tests {
		if got := MapGPUContextKey(tc.key); got != tc.want {
			t.Fatalf("%s mapped to %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestPollingKeyMapCoversQuakeCriticalKeys(t *testing.T) {
	got := make(map[ginput.Key]int, len(PollingKeyMap))
	for _, pair := range PollingKeyMap {
		got[pair.Src] = pair.Dst
	}

	tests := []struct {
		name string
		key  ginput.Key
		want int
	}{
		{name: "grave", key: ginput.KeyGrave, want: int('`')},
		{name: "apostrophe", key: ginput.KeyApostrophe, want: int('\'')},
		{name: "minus", key: ginput.KeyMinus, want: int('-')},
		{name: "left bracket", key: ginput.KeyLeftBracket, want: int('[')},
		{name: "scroll lock", key: ginput.KeyScrollLock, want: iinput.KScrollLock},
		{name: "page up", key: ginput.KeyPageUp, want: iinput.KPgUp},
		{name: "f1", key: ginput.KeyF1, want: iinput.KF1},
		{name: "numpad 0", key: ginput.KeyNumpad0, want: iinput.KKpIns},
		{name: "num lock", key: ginput.KeyNumLock, want: iinput.KKpNumLock},
	}

	for _, tc := range tests {
		if got[tc.key] != tc.want {
			t.Fatalf("%s mapped to %d, want %d", tc.name, got[tc.key], tc.want)
		}
	}
}

func TestInputBackendSetMouseGrabSetsCursorMode(t *testing.T) {
	var backend InputBackend

	backend.SetMouseGrab(true)
	if backend.cursorMode != iinput.CursorModeGrabbed {
		t.Fatalf("cursorMode after grab = %v, want grabbed", backend.cursorMode)
	}

	backend.SetMouseGrab(false)
	if backend.cursorMode != iinput.CursorModeNormal {
		t.Fatalf("cursorMode after release = %v, want normal", backend.cursorMode)
	}
}

func TestCursorModeAdapter(t *testing.T) {
	tests := []struct {
		name           string
		mode           iinput.CursorMode
		wantCursor     gpucontext.CursorShape
		wantCursorMode gpucontext.CursorMode
	}{
		{
			name:           "normal",
			mode:           iinput.CursorModeNormal,
			wantCursor:     gpucontext.CursorDefault,
			wantCursorMode: gpucontext.CursorModeNormal,
		},
		{
			name:           "hidden",
			mode:           iinput.CursorModeHidden,
			wantCursor:     gpucontext.CursorNone,
			wantCursorMode: gpucontext.CursorModeNormal,
		},
		{
			name:           "grabbed",
			mode:           iinput.CursorModeGrabbed,
			wantCursor:     gpucontext.CursorNone,
			wantCursorMode: gpucontext.CursorModeLocked,
		},
	}

	for _, tc := range tests {
		cursor, cursorMode := cursorModeAdapter(tc.mode)
		if cursor != tc.wantCursor || cursorMode != tc.wantCursorMode {
			t.Fatalf("%s adapter = (%v, %v), want (%v, %v)", tc.name, cursor, cursorMode, tc.wantCursor, tc.wantCursorMode)
		}
	}
}
