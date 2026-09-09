package eval

import (
	"path/filepath"
	"testing"
)

func TestManifestRoundTripAndUpsert(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raw", "manifest.jsonl")
	a := ManifestEntry{
		PkgID: "pkg-a", LicenseNote: "GPL-2.0",
		MapFiles: []string{"a.map"}, Flags: Flags{ToolchainGuess: "ericw"},
	}
	b := ManifestEntry{
		PkgID: "pkg-b", LicenseNote: "synthetic",
		MapFiles: []string{"b.map"}, BSPFiles: []string{"b.bsp"},
	}
	if err := AppendToManifest(path, []ManifestEntry{a, b}); err != nil {
		t.Fatalf("AppendToManifest: %v", err)
	}
	entries, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	// upsert a with new flags: count stable, value replaced
	a.Flags.Era = "modern"
	if err := AppendToManifest(path, []ManifestEntry{a}); err != nil {
		t.Fatalf("AppendToManifest: %v", err)
	}
	entries, err = LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d after upsert, want 2", len(entries))
	}
	if entries[0].Flags.Era != "modern" {
		t.Fatalf("upsert did not replace: %+v", entries[0].Flags)
	}
}

func TestManifestMissingReturnsNil(t *testing.T) {
	entries, err := LoadManifest(filepath.Join(t.TempDir(), "nope.jsonl"))
	if err != nil || entries != nil {
		t.Fatalf("LoadManifest = %v, %v; want nil, nil", entries, err)
	}
}

func TestValidateEntry(t *testing.T) {
	bad := []ManifestEntry{
		{PkgID: "", LicenseNote: "x", MapFiles: []string{"a.map"}},
		{PkgID: "p", LicenseNote: "", MapFiles: []string{"a.map"}},
		{PkgID: "p", LicenseNote: "x"},
		{PkgID: "p", LicenseNote: "x", MapFiles: []string{"../escape.map"}},
	}
	for i, e := range bad {
		if err := ValidateEntry(&e); err == nil {
			t.Fatalf("case %d: expected validation error", i)
		}
	}
	good := ManifestEntry{PkgID: "p", LicenseNote: "MIT", MapFiles: []string{"a.map"}}
	if err := ValidateEntry(&good); err != nil {
		t.Fatalf("valid entry rejected: %v", err)
	}
}
