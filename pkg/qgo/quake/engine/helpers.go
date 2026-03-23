package engine

// CRandom returns a random float value between -1.0 and 1.0.
func CRandom() float32 { return Random()*2 - 1 }
