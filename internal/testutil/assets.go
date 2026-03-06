package testutil

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// LocateQuakeDir attempts to find the Quake installation directory containing id1.
func LocateQuakeDir() (string, error) {
	// Check environment variable first
	if envPath := os.Getenv("QUAKE_DIR"); envPath != "" {
		id1Path := filepath.Join(envPath, "id1")
		if stat, err := os.Stat(id1Path); err == nil && stat.IsDir() {
			return envPath, nil
		}
	}

	// Common relative paths to check for id1 directory
	paths := []string{
		".",
		"..",
		"../..",
		"../../..",
	}

	for _, p := range paths {
		// Check if p itself is id1
		if filepath.Base(p) == "id1" {
			if stat, err := os.Stat(p); err == nil && stat.IsDir() {
				abs, err := filepath.Abs(filepath.Dir(p))
				if err == nil {
					return abs, nil
				}
				return filepath.Dir(p), nil
			}
		}
		// Check if p contains id1
		id1Path := filepath.Join(p, "id1")
		if stat, err := os.Stat(id1Path); err == nil && stat.IsDir() {
			abs, err := filepath.Abs(p)
			if err == nil {
				return abs, nil
			}
			return p, nil
		}
	}

	return "", fmt.Errorf("quake directory (containing id1) not found")
}

// LocatePak0 attempts to find pak0.pak in common locations.
func LocatePak0() (string, error) {
	if envPath := os.Getenv("QUAKE_PAK0_PATH"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath, nil
		}
	}

	qdir, err := LocateQuakeDir()
	if err == nil {
		pakPath := filepath.Join(qdir, "id1", "pak0.pak")
		if _, err := os.Stat(pakPath); err == nil {
			return pakPath, nil
		}
	}

	return "", fmt.Errorf("pak0.pak not found")
}

// SkipIfNoQuakeDir skips the test if the Quake directory cannot be located.
func SkipIfNoQuakeDir(t *testing.T) string {
	t.Helper()
	path, err := LocateQuakeDir()
	if err != nil {
		t.Skipf("Skipping test: Quake directory not found: %v", err)
	}
	return path
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
