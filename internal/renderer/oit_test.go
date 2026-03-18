package renderer

import "testing"

func TestOITFramebuffersZeroValue(t *testing.T) {
	var fb oitFramebuffers
	if fb.fbo != 0 {
		t.Fatalf("zero-value oitFramebuffers.fbo = %d, want 0", fb.fbo)
	}
}

func TestShouldUseOITResources(t *testing.T) {
	ensureAlphaModeCvars()

	cvarScenarios := []struct {
		name      string
		oit       bool
		alphaSort bool
		want      bool
	}{
		{name: "oit enabled", oit: true, alphaSort: false, want: true},
		{name: "sorted only", oit: false, alphaSort: true, want: false},
		{name: "basic", oit: false, alphaSort: false, want: false},
	}

	for _, tc := range cvarScenarios {
		t.Run(tc.name, func(t *testing.T) {
			SetAlphaMode(AlphaModeBasic)
			if tc.alphaSort {
				SetAlphaMode(AlphaModeSorted)
			}
			if tc.oit {
				SetAlphaMode(AlphaModeOIT)
			}

			gotMode := GetAlphaMode()
			gotUseOIT := ShouldUseOITResources()
			wantMode := AlphaModeBasic
			if tc.alphaSort {
				wantMode = AlphaModeSorted
			}
			if tc.oit {
				wantMode = AlphaModeOIT
			}

			if gotMode != wantMode {
				t.Fatalf("GetAlphaMode() = %v, want %v", gotMode, wantMode)
			}
			if gotUseOIT != tc.want {
				t.Fatalf("ShouldUseOITResources() = %v, want %v", gotUseOIT, tc.want)
			}
		})
	}
}
