//go:build !(js && wasm)

package gogpu

import "github.com/gogpu/gputypes"

// GetBrowserPreferredCanvasFormat returns BGRA8Unorm on non-WASM platforms.
func GetBrowserPreferredCanvasFormat() gputypes.TextureFormat {
	return gputypes.TextureFormatBGRA8Unorm
}
