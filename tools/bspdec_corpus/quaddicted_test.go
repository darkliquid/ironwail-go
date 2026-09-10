package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestQuaddictedRecordParsing(t *testing.T) {
	rec := QuaddictedRecord{
		SHA256: "0057e13199459fb3e0450484ada072fe2c7c64f6f75e38b8e314ac2808bd0a4d",
		Bytes:  1558519,
		Tags: []string{
			"author=Rastrelly",
			"filename=cotdq1sp.zip",
			"title=Cathedral of the Doubt",
		},
		URLs: []string{
			"https://www.quaddicted.com/filebase/cotdq1sp.zip",
		},
		Files: map[string]struct {
			Bytes     int64  `json:"bytes"`
			SHA256    string `json:"sha256"`
			Timestamp string `json:"timestamp"`
		}{
			"cotdq1sp.bsp": {Bytes: 3692200},
			"cotdq1sp.map": {Bytes: 1300696},
			"cotdq1.txt":   {Bytes: 1438},
		},
	}

	if rec.TagValue("author") != "Rastrelly" {
		t.Errorf("author = %q, want Rastrelly", rec.TagValue("author"))
	}
	if rec.TagValue("title") != "Cathedral of the Doubt" {
		t.Errorf("title = %q, want Cathedral of the Doubt", rec.TagValue("title"))
	}
	if rec.PkgID() != "cotdq1sp" {
		t.Errorf("PkgID = %q, want cotdq1sp", rec.PkgID())
	}
	if !rec.HasPairedMaps() {
		t.Errorf("expected HasPairedMaps = true")
	}

	// Unpaired record (map only)
	recMapOnly := QuaddictedRecord{
		Files: map[string]struct {
			Bytes     int64  `json:"bytes"`
			SHA256    string `json:"sha256"`
			Timestamp string `json:"timestamp"`
		}{
			"only.map": {Bytes: 100},
		},
	}
	if recMapOnly.HasPairedMaps() {
		t.Errorf("expected HasPairedMaps = false for map-only")
	}
}

func TestExtractQuaddictedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "testpkg.zip")

	// Create a zip with .map, .bsp, .txt, and an ignored .wav
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	files := map[string]string{
		"maps/test.map": "worldspawn {}",
		"maps/test.bsp": "BSPX fake data",
		"readme.txt":    "Map readme",
		"sound/fx.wav":  "audio data",
	}
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(zipPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	extractTarget := filepath.Join(tmpDir, "extracted")
	maps, bsps, err := extractQuaddictedFiles(zipPath, extractTarget)
	if err != nil {
		t.Fatalf("extractQuaddictedFiles: %v", err)
	}

	if len(maps) != 1 || filepath.Base(maps[0]) != "test.map" {
		t.Errorf("maps = %v, want [test.map]", maps)
	}
	if len(bsps) != 1 || filepath.Base(bsps[0]) != "test.bsp" {
		t.Errorf("bsps = %v, want [test.bsp]", bsps)
	}

	// Check that readme was extracted and wav was ignored
	if _, err := os.Stat(filepath.Join(extractTarget, "readme.txt")); err != nil {
		t.Errorf("expected readme.txt to be extracted")
	}
	if _, err := os.Stat(filepath.Join(extractTarget, "fx.wav")); err == nil {
		t.Errorf("expected fx.wav to NOT be extracted")
	}
}

func TestScanQuaddictedCatalogMock(t *testing.T) {
	tmpDir := t.TempDir()
	jsonDir := filepath.Join(tmpDir, "json", "by-sha256", "aa")
	if err := os.MkdirAll(jsonDir, 0o755); err != nil {
		t.Fatal(err)
	}

	pairedRec := QuaddictedRecord{
		SHA256: "aabbccdd11223344",
		Tags:   []string{"filename=mymap.zip"},
		Files: map[string]struct {
			Bytes     int64  `json:"bytes"`
			SHA256    string `json:"sha256"`
			Timestamp string `json:"timestamp"`
		}{
			"mymap.map": {},
			"mymap.bsp": {},
		},
	}
	unpairedRec := QuaddictedRecord{
		SHA256: "eeff001122334455",
		Tags:   []string{"filename=bsp_only.zip"},
		Files: map[string]struct {
			Bytes     int64  `json:"bytes"`
			SHA256    string `json:"sha256"`
			Timestamp string `json:"timestamp"`
		}{
			"bsp_only.bsp": {},
		},
	}

	data1, _ := json.Marshal(pairedRec)
	data2, _ := json.Marshal(unpairedRec)
	if err := os.WriteFile(filepath.Join(jsonDir, "rec1.json"), data1, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jsonDir, "rec2.json"), data2, 0o644); err != nil {
		t.Fatal(err)
	}

	scanned, err := ScanQuaddictedCatalog(tmpDir)
	if err != nil {
		t.Fatalf("ScanQuaddictedCatalog: %v", err)
	}
	if len(scanned) != 1 {
		t.Fatalf("scanned = %d records, want 1", len(scanned))
	}
	if scanned[0].PkgID() != "mymap" {
		t.Errorf("scanned PkgID = %q, want mymap", scanned[0].PkgID())
	}
}
