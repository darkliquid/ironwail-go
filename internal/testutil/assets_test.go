package testutil

import (
	"os"
	"testing"
)

func TestLocatePak0(t *testing.T) {
	path, err := LocatePak0()
	if err != nil {
		t.Logf("pak0.pak not found (this is expected if not present): %v", err)
	} else {
		t.Logf("Found pak0.pak at: %s", path)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("LocatePak0 returned a path that doesn't exist: %s", path)
		}
	}
}

func TestCompareStructs(t *testing.T) {
	t.Run("EqualInts", func(t *testing.T) {
		CompareStructs(t, 1, 1)
	})

	t.Run("EqualByteSlices", func(t *testing.T) {
		CompareStructs(t, []byte{1, 2, 3}, []byte{1, 2, 3})
	})
}

func TestAssertNoError(t *testing.T) {
	AssertNoError(t, nil)
}

func TestSkipIfNoPak0(t *testing.T) {
	// This will either return a path or skip the test.
	// Since we can't easily verify the skip without complex mocking,
	// we just call it to ensure it doesn't panic.
	path := SkipIfNoPak0(t)
	if path != "" {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("SkipIfNoPak0 returned a path that doesn't exist: %s", path)
		}
	}
}
