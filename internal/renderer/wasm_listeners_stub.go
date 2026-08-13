//go:build !(js && wasm)

package renderer

func attachWasmDeviceListeners(deviceObj any) {}
