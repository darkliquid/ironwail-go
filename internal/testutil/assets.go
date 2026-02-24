package testutil

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// LocatePak0 attempts to find pak0.pak in common locations.
func LocatePak0() (string, error) {
	// Check environment variable first
	if envPath := os.Getenv("QUAKE_PAK0_PATH"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath, nil
		}
	}

	// Common relative paths
	paths := []string{
		"id1/pak0.pak",
		"../id1/pak0.pak",
		"../../id1/pak0.pak",
		"../../../id1/pak0.pak",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			abs, err := filepath.Abs(p)
			if err == nil {
				return abs, nil
			}
			return p, nil
		}
	}

	return "", fmt.Errorf("pak0.pak not found")
}

// SkipIfNoPak0 skips the test if pak0.pak cannot be located.
func SkipIfNoPak0(t *testing.T) string {
	t.Helper()
	path, err := LocatePak0()
	if err != nil {
		t.Skipf("Skipping test: pak0.pak not found: %v", err)
	}
	return path
}

// CompareStructs compares two structs and fails the test if they are not equal.
// It provides a basic hex dump if they differ and are byte slices.
func CompareStructs(t *testing.T, expected, actual interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		// If they are byte slices, show hex dump
		expBytes, ok1 := expected.([]byte)
		actBytes, ok2 := actual.([]byte)
		if ok1 && ok2 {
			t.Errorf("Byte slices differ.\nExpected:\n%s\nActual:\n%s", hex.Dump(expBytes), hex.Dump(actBytes))
		} else {
			t.Errorf("Objects differ.\nExpected: %+v\nActual:   %+v", expected, actual)
		}
	}
}

// AssertNoError is a helper to fail a test if an error is not nil.
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}
