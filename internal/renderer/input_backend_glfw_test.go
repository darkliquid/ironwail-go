//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"testing"

	"github.com/go-gl/glfw/v3.3/glfw"
	iinput "github.com/ironwail/ironwail-go/internal/input"
	openglimpl "github.com/ironwail/ironwail-go/internal/renderer/opengl"
)

func TestMapGLFWKeyMapsPunctuationAndSpecialKeys(t *testing.T) {
	tests := []struct {
		name string
		key  glfw.Key
		want int
	}{
		{name: "grave", key: glfw.KeyGraveAccent, want: int('`')},
		{name: "apostrophe", key: glfw.KeyApostrophe, want: int('\'')},
		{name: "comma", key: glfw.KeyComma, want: int(',')},
		{name: "minus", key: glfw.KeyMinus, want: int('-')},
		{name: "period", key: glfw.KeyPeriod, want: int('.')},
		{name: "slash", key: glfw.KeySlash, want: int('/')},
		{name: "semicolon", key: glfw.KeySemicolon, want: int(';')},
		{name: "equal", key: glfw.KeyEqual, want: int('=')},
		{name: "left bracket", key: glfw.KeyLeftBracket, want: int('[')},
		{name: "backslash", key: glfw.KeyBackslash, want: int('\\')},
		{name: "right bracket", key: glfw.KeyRightBracket, want: int(']')},
		{name: "scroll lock", key: glfw.KeyScrollLock, want: iinput.KScrollLock},
		{name: "print screen", key: glfw.KeyPrintScreen, want: iinput.KPrintScreen},
		{name: "left super", key: glfw.KeyLeftSuper, want: iinput.KCommand},
	}

	for _, tc := range tests {
		if got := openglimpl.MapGLFWKey(tc.key); got != tc.want {
			t.Fatalf("%s mapped to %d, want %d", tc.name, got, tc.want)
		}
	}
}
