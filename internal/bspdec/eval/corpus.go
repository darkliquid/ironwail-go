package eval

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Flags captures per-package corpus metadata (spec section 9.1).
type Flags struct {
	Brushlist      bool   `json:"brushlist"`
	ToolchainGuess string `json:"toolchain_guess,omitempty"`
	Era            string `json:"era,omitempty"`
	ClassicHoldout bool   `json:"classic_holdout,omitempty"`
}

// ManifestEntry is one line of the corpus manifest (spec section 9.1
// schema): a package with its map/bsp files and licensing note.
type ManifestEntry struct {
	PkgID       string   `json:"pkg_id"`
	SourceURL   string   `json:"source_url,omitempty"`
	SHA256      string   `json:"sha256,omitempty"`
	LicenseNote string   `json:"license_note"`
	MapFiles    []string `json:"map_files"`
	BSPFiles    []string `json:"bsp_files"`
	Flags       Flags    `json:"flags"`
}

// LoadManifest reads the JSONL manifest; returns nil, nil when absent.
func LoadManifest(path string) ([]ManifestEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var entries []ManifestEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	line := 0
	for sc.Scan() {
		line++
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		var e ManifestEntry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("manifest line %d: %w", line, err)
		}
		entries = append(entries, e)
	}
	return entries, sc.Err()
}

// AppendToManifest upserts entries by PkgID (idempotent re-runs), preserving
// file order, then rewrites the manifest atomically.
func AppendToManifest(path string, entries []ManifestEntry) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	existing, err := LoadManifest(path)
	if err != nil {
		return err
	}
	idx := map[string]int{}
	for i, e := range existing {
		idx[e.PkgID] = i
	}
	for _, e := range entries {
		if i, ok := idx[e.PkgID]; ok {
			existing[i] = e // update in place
			continue
		}
		idx[e.PkgID] = len(existing)
		existing = append(existing, e)
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	fail := func(err error) error {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	enc := json.NewEncoder(f)
	for _, e := range existing {
		if err := enc.Encode(e); err != nil {
			return fail(err)
		}
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// ValidateEntry checks the manifest contract: non-empty PkgID and
// LicenseNote, at least one map or bsp file, and no path escapes.
func ValidateEntry(e *ManifestEntry) error {
	if e.PkgID == "" {
		return fmt.Errorf("empty pkg_id")
	}
	if e.LicenseNote == "" {
		return fmt.Errorf("pkg %q: missing license_note", e.PkgID)
	}
	if len(e.MapFiles) == 0 && len(e.BSPFiles) == 0 {
		return fmt.Errorf("pkg %q: no map or bsp files", e.PkgID)
	}
	for _, f := range append(append([]string{}, e.MapFiles...), e.BSPFiles...) {
		if strings.Contains(f, "..") || filepath.IsAbs(f) {
			return fmt.Errorf("pkg %q: unsafe path %q", e.PkgID, f)
		}
	}
	return nil
}