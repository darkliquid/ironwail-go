package renderer

import "testing"

func TestTranslatePlayerSkinPixelsRemapsTopAndBottomRanges(t *testing.T) {
	input := []byte{0, 16, 31, 96, 111, 200}
	got := TranslatePlayerSkinPixels(input, 4, 9)

	want := []byte{0, 64, 79, 159, 144, 200}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pixel[%d] = %d, want %d", i, got[i], want[i])
		}
	}
	if input[1] != 16 || input[3] != 96 {
		t.Fatal("TranslatePlayerSkinPixels modified source slice")
	}
}

func TestTranslatePlayerSkinPixelsUsesReversedHighPaletteRows(t *testing.T) {
	got := TranslatePlayerSkinPixels([]byte{16, 31, 96, 111}, 12, 15)
	want := []byte{207, 192, 255, 240}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pixel[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}
