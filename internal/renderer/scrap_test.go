package renderer

import (
	"reflect"
	"testing"
)

func TestScrapAllocatorBasicAllocation(t *testing.T) {
	a := NewScrapAllocator(8, 8)

	x, y, ok := a.Alloc(3, 2)
	if !ok {
		t.Fatal("Alloc(3,2) failed")
	}
	if x != 0 || y != 0 {
		t.Fatalf("Alloc(3,2) = (%d,%d), want (0,0)", x, y)
	}

	wantSkyline := []int{2, 2, 2, 0, 0, 0, 0, 0}
	if !reflect.DeepEqual(a.skyline, wantSkyline) {
		t.Fatalf("skyline = %v, want %v", a.skyline, wantSkyline)
	}
	if got := a.UsedHeight(); got != 2 {
		t.Fatalf("UsedHeight() = %d, want 2", got)
	}
	if got, want := a.FreeArea(), 58; got != want {
		t.Fatalf("FreeArea() = %d, want %d", got, want)
	}
}

func TestScrapAllocatorPacksMultipleAllocations(t *testing.T) {
	a := NewScrapAllocator(8, 8)

	cases := []struct {
		w, h         int
		wantX, wantY int
	}{
		{3, 2, 0, 0},
		{3, 2, 3, 0},
		{2, 2, 6, 0},
		{4, 2, 0, 2},
	}

	for i, tc := range cases {
		x, y, ok := a.Alloc(tc.w, tc.h)
		if !ok {
			t.Fatalf("alloc %d failed for %dx%d", i, tc.w, tc.h)
		}
		if x != tc.wantX || y != tc.wantY {
			t.Fatalf("alloc %d got (%d,%d), want (%d,%d)", i, x, y, tc.wantX, tc.wantY)
		}
	}

	wantSkyline := []int{4, 4, 4, 4, 2, 2, 2, 2}
	if !reflect.DeepEqual(a.skyline, wantSkyline) {
		t.Fatalf("skyline = %v, want %v", a.skyline, wantSkyline)
	}
}

func TestScrapAllocatorOverflow(t *testing.T) {
	a := NewScrapAllocator(4, 4)

	if _, _, ok := a.Alloc(4, 4); !ok {
		t.Fatal("expected 4x4 allocation to succeed")
	}
	if _, _, ok := a.Alloc(1, 1); ok {
		t.Fatal("expected overflow allocation to fail")
	}
}

func TestScrapAllocatorEdgeCases(t *testing.T) {
	t.Run("full width", func(t *testing.T) {
		a := NewScrapAllocator(8, 8)
		x, y, ok := a.Alloc(8, 3)
		if !ok || x != 0 || y != 0 {
			t.Fatalf("Alloc(8,3) = (%d,%d,%v), want (0,0,true)", x, y, ok)
		}
	})

	t.Run("full height", func(t *testing.T) {
		a := NewScrapAllocator(8, 8)
		x, y, ok := a.Alloc(2, 8)
		if !ok || x != 0 || y != 0 {
			t.Fatalf("Alloc(2,8) = (%d,%d,%v), want (0,0,true)", x, y, ok)
		}
		if _, _, ok := a.Alloc(2, 1); !ok {
			t.Fatal("expected allocation in remaining columns")
		}
	})

	t.Run("single pixel", func(t *testing.T) {
		a := NewScrapAllocator(2, 2)
		x, y, ok := a.Alloc(1, 1)
		if !ok || x != 0 || y != 0 {
			t.Fatalf("Alloc(1,1) = (%d,%d,%v), want (0,0,true)", x, y, ok)
		}
	})

	t.Run("invalid sizes", func(t *testing.T) {
		a := NewScrapAllocator(8, 8)
		invalid := [][2]int{{0, 1}, {1, 0}, {-1, 1}, {1, -1}, {9, 1}, {1, 9}}
		for _, v := range invalid {
			if _, _, ok := a.Alloc(v[0], v[1]); ok {
				t.Fatalf("Alloc(%d,%d) should fail", v[0], v[1])
			}
		}
	})
}

func TestScrapAllocatorReset(t *testing.T) {
	a := NewScrapAllocator(8, 8)
	if _, _, ok := a.Alloc(3, 3); !ok {
		t.Fatal("first alloc failed")
	}
	if _, _, ok := a.Alloc(2, 2); !ok {
		t.Fatal("second alloc failed")
	}

	a.Reset()

	want := make([]int, 8)
	if !reflect.DeepEqual(a.skyline, want) {
		t.Fatalf("skyline after reset = %v, want %v", a.skyline, want)
	}
	if got := a.UsedHeight(); got != 0 {
		t.Fatalf("UsedHeight() after reset = %d, want 0", got)
	}
	if got, want := a.FreeArea(), 64; got != want {
		t.Fatalf("FreeArea() after reset = %d, want %d", got, want)
	}
}

func TestScrapAllocatorSkylineTracking(t *testing.T) {
	a := NewScrapAllocator(6, 8)

	a.Alloc(2, 2) // x=0,y=0
	a.Alloc(2, 3) // x=2,y=0
	a.Alloc(2, 1) // x=4,y=0
	a.Alloc(1, 2) // x=4,y=1 best low column

	wantSkyline := []int{2, 2, 3, 3, 3, 1}
	if !reflect.DeepEqual(a.skyline, wantSkyline) {
		t.Fatalf("skyline = %v, want %v", a.skyline, wantSkyline)
	}
	if got := a.UsedHeight(); got != 3 {
		t.Fatalf("UsedHeight() = %d, want 3", got)
	}
}

func TestScrapAllocatorBestFitGapSelection(t *testing.T) {
	a := NewScrapAllocator(8, 8)

	if _, _, ok := a.Alloc(2, 4); !ok { // skyline [4 4 0 0 0 0 0 0]
		t.Fatal("alloc 1 failed")
	}
	if _, _, ok := a.Alloc(2, 2); !ok { // skyline [4 4 2 2 0 0 0 0]
		t.Fatal("alloc 2 failed")
	}
	if _, _, ok := a.Alloc(2, 4); !ok { // skyline [4 4 2 2 4 4 0 0]
		t.Fatal("alloc 3 failed")
	}

	x, y, ok := a.Alloc(2, 2)
	if !ok {
		t.Fatal("gap allocation failed")
	}
	if x != 6 || y != 0 {
		t.Fatalf("gap allocation = (%d,%d), want (6,0)", x, y)
	}
}
