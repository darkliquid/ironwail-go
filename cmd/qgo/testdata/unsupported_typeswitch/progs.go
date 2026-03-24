package main

func TypeSwitchValue(v any) float32 {
	switch x := v.(type) {
	case float32:
		return x
	default:
		return 0
	}
}
