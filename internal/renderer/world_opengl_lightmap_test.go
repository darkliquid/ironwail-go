package renderer

import "testing"

func TestExpandLightmapSamplesExpandsMonoToRGB(t *testing.T) {
	if got, want := expandLightmapSamples([]byte{12}, false, 0, 1), []byte{12, 12, 12}; string(got) != string(want) {
		t.Fatalf("samples = %v, want %v", got, want)
	}
}

func TestExpandLightmapSamplesPreservesRGBTriplets(t *testing.T) {
	if got, want := expandLightmapSamples([]byte{1, 2, 3}, true, 0, 1), []byte{1, 2, 3}; string(got) != string(want) {
		t.Fatalf("samples = %v, want %v", got, want)
	}
}
