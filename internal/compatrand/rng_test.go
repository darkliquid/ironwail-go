package compatrand

import "testing"

func TestSequenceMatchesCLibRand(t *testing.T) {
	rng := New()
	want := []int32{
		1804289383,
		846930886,
		1681692777,
		1714636915,
		1957747793,
	}

	for i, wantValue := range want {
		if got := rng.Int(); got != wantValue {
			t.Fatalf("value %d = %d, want %d", i, got, wantValue)
		}
	}
}

func TestSeedZeroMatchesDefaultSeed(t *testing.T) {
	defaultRNG := New()
	zeroSeedRNG := NewSeed(0)

	for i := 0; i < 3; i++ {
		if got, want := zeroSeedRNG.Int(), defaultRNG.Int(); got != want {
			t.Fatalf("value %d = %d, want %d", i, got, want)
		}
	}
}
