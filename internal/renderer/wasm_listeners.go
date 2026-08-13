//go:build js && wasm

package renderer

import (
	"log/slog"
	"reflect"
	"syscall/js"
)

var wasmListenersAttached = false

// attachWasmDeviceListeners attaches WebGPU onuncapturederror and device.lost
// event handlers to the underlying browser GPUDevice.
func attachWasmDeviceListeners(deviceObj any) {
	if wasmListenersAttached || deviceObj == nil {
		return
	}

	jsDev := extractJSValue(deviceObj)
	if jsDev.IsUndefined() || jsDev.IsNull() {
		slog.Warn("WASM renderer: could not extract JS GPUDevice for error listener attachment")
		return
	}

	wasmListenersAttached = true
	slog.Info("WASM renderer: attaching WebGPU onuncapturederror and device.lost listeners")

	// 1. onuncapturederror listener
	uncapturedCb := js.FuncOf(func(this js.Value, args []js.Value) any {
		errMsg := "unknown WebGPU error"
		if len(args) > 0 && !args[0].IsUndefined() && !args[0].IsNull() {
			errObj := args[0].Get("error")
			if !errObj.IsUndefined() && !errObj.IsNull() {
				errMsg = errObj.Get("message").String()
			} else {
				errMsg = args[0].String()
			}
		}
		slog.Error("🚨 WebGPU Uncaptured Error", "error", errMsg)
		return nil
	})
	jsDev.Set("onuncapturederror", uncapturedCb)

	// 2. device.lost promise listener
	lostProp := jsDev.Get("lost")
	if !lostProp.IsUndefined() && !lostProp.IsNull() && lostProp.Get("then").Type() == js.TypeFunction {
		lostCb := js.FuncOf(func(this js.Value, args []js.Value) any {
			reason := "unknown"
			msg := ""
			if len(args) > 0 && !args[0].IsUndefined() && !args[0].IsNull() {
				if r := args[0].Get("reason"); !r.IsUndefined() {
					reason = r.String()
				}
				if m := args[0].Get("message"); !m.IsUndefined() {
					msg = m.String()
				}
			}
			slog.Error("🚨 WebGPU Device Lost", "reason", reason, "message", msg)
			return nil
		})
		lostProp.Call("then", lostCb)
	}
}

func extractJSValue(v any) js.Value {
	if v == nil {
		return js.Undefined()
	}
	if jsv, ok := v.(js.Value); ok {
		return jsv
	}

	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Pointer {
		if val.IsNil() {
			return js.Undefined()
		}
		val = val.Elem()
	}

	if val.Kind() == reflect.Struct {
		jsValType := reflect.TypeOf(js.Value{})
		for i := 0; i < val.NumField(); i++ {
			field := val.Field(i)
			if field.Type() == jsValType {
				jsv := field.Interface().(js.Value)
				if !jsv.IsUndefined() && !jsv.IsNull() {
					return jsv
				}
			}
		}
	}
	return js.Undefined()
}
