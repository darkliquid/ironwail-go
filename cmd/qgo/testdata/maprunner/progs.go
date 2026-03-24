package progs

func MapRunner(start, steps float32) float32 {
	pos := start
	var i float32
	for i = 0; i < steps; i++ {
		if pos > 5 {
			pos = pos - 2
		} else {
			pos = pos + 3
		}
	}
	return pos
}
