package oit

import (
	"testing"
)

func TestOITHelpers(t *testing.T) {
	if !ShouldUseResources(AlphaModeOIT) {
		t.Errorf("ShouldUseResources(%d) = false, want true", AlphaModeOIT)
	}
	if ShouldUseResources(0) {
		t.Errorf("ShouldUseResources(0) = true, want false")
	}

	if ShouldSortTranslucentCalls(AlphaModeOIT) {
		t.Errorf("ShouldSortTranslucentCalls(%d) = true, want false", AlphaModeOIT)
	}
	if !ShouldSortTranslucentCalls(0) {
		t.Errorf("ShouldSortTranslucentCalls(0) = false, want true")
	}
}
