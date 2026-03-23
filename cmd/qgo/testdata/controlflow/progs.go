package progs

func Max(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func Sum(n float32) float32 {
	var result float32
	var i float32
	for i = 0; i < n; i++ {
		result = result + i
	}
	return result
}
