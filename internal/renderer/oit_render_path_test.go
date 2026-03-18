package renderer

import "testing"

func TestShouldSortTranslucentCallsByAlphaMode(t *testing.T) {
	tests := []struct {
		name string
		mode AlphaMode
		want bool
	}{
		{name: "basic mode sorts", mode: AlphaModeBasic, want: true},
		{name: "sorted mode sorts", mode: AlphaModeSorted, want: true},
		{name: "oit mode skips sort", mode: AlphaModeOIT, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldSortTranslucentCalls(tc.mode); got != tc.want {
				t.Fatalf("shouldSortTranslucentCalls(%v) = %v, want %v", tc.mode, got, tc.want)
			}
		})
	}
}

func TestRenderPathDecisionsFollowAlphaMode(t *testing.T) {
	ensureAlphaModeCvars()

	tests := []struct {
		name     string
		setMode  AlphaMode
		wantOIT  bool
		wantSort bool
	}{
		{name: "basic", setMode: AlphaModeBasic, wantOIT: false, wantSort: true},
		{name: "sorted", setMode: AlphaModeSorted, wantOIT: false, wantSort: true},
		{name: "oit", setMode: AlphaModeOIT, wantOIT: true, wantSort: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			SetAlphaMode(tc.setMode)
			mode := GetAlphaMode()

			if got := ShouldUseOITResources(); got != tc.wantOIT {
				t.Fatalf("ShouldUseOITResources() = %v, want %v", got, tc.wantOIT)
			}
			if got := shouldSortTranslucentCalls(mode); got != tc.wantSort {
				t.Fatalf("shouldSortTranslucentCalls(GetAlphaMode()) = %v, want %v", got, tc.wantSort)
			}
		})
	}
}
