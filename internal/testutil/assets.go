package testutil

import (
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const Pak0URL = "https://github.com/pweil-/origin-quake/raw/refs/heads/master/id1/pak0.pak"

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
		"testdata",
		"quake-data",
		"..",
		"../testdata",
		"../quake-data",
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

// EnsurePak0 checks if pak0.pak exists locally using LocatePak0. If not found,
// it downloads official pak0.pak from Pak0URL on demand and caches it in
// ./testdata/id1/pak0.pak.
func EnsurePak0() (string, error) {
	path, err := LocatePak0()
	if err == nil {
		return path, nil
	}

	targetDir := filepath.Join("testdata", "id1")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("create %s dir: %w", targetDir, err)
	}
	targetFile := filepath.Join(targetDir, "pak0.pak")

	if _, err := os.Stat(targetFile); err == nil {
		return targetFile, nil
	}

	slog.Info("pak0.pak not found locally; downloading official minimal pak0.pak...", "url", Pak0URL, "target", targetFile)
	resp, err := http.Get(Pak0URL)
	if err != nil {
		return "", fmt.Errorf("download pak0.pak from %s failed: %w", Pak0URL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download pak0.pak HTTP status %d", resp.StatusCode)
	}

	tmpFile := targetFile + ".tmp"
	out, err := os.Create(tmpFile)
	if err != nil {
		return "", fmt.Errorf("create tmp pak0 file: %w", err)
	}

	_, copyErr := io.Copy(out, resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmpFile)
		return "", fmt.Errorf("download body copy failed: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpFile)
		return "", fmt.Errorf("close tmp pak0 file: %w", closeErr)
	}

	if err := os.Rename(tmpFile, targetFile); err != nil {
		return "", fmt.Errorf("rename tmp pak0 file: %w", err)
	}

	slog.Info("successfully downloaded and cached pak0.pak", "path", targetFile)
	return targetFile, nil
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

// SkipIfNoPak0 ensures pak0.pak exists (downloading it on demand if needed).
func SkipIfNoPak0(t *testing.T) string {
	t.Helper()
	path, err := EnsurePak0()
	if err != nil {
		t.Skipf("Skipping test: pak0.pak not available and download failed: %v", err)
	}
	return path
}

// CompareStructs compares two structs and fails the test if they are not equal.
// It provides a basic hex dump if they differ and are byte slices.
func CompareStructs(t *testing.T, expected, actual any) {
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
