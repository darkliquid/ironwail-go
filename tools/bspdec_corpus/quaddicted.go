package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/darkliquid/ironwail-go/internal/bspdec/eval"
)

// QuaddictedRecord describes the structure of quaddicted-data JSON metadata.
type QuaddictedRecord struct {
	Bytes       int64  `json:"bytes"`
	Description string `json:"description"`
	Files       map[string]struct {
		Bytes     int64  `json:"bytes"`
		SHA256    string `json:"sha256"`
		Timestamp string `json:"timestamp"`
	} `json:"files"`
	SHA256 string   `json:"sha256"`
	Tags   []string `json:"tags"`
	URLs   []string `json:"urls"`
}

// TagValue extracts the value for a "key=val" tag.
func (r *QuaddictedRecord) TagValue(key string) string {
	prefix := key + "="
	for _, t := range r.Tags {
		if strings.HasPrefix(t, prefix) {
			return strings.TrimPrefix(t, prefix)
		}
	}
	return ""
}

// PkgID computes a clean package ID from the filename tag or sha256.
func (r *QuaddictedRecord) PkgID() string {
	fn := r.TagValue("filename")
	if fn != "" {
		stem := strings.TrimSuffix(fn, filepath.Ext(fn))
		if stem != "" {
			return stem
		}
	}
	if len(r.SHA256) >= 12 {
		return r.SHA256[:12]
	}
	return "pkg-" + r.SHA256
}

// HasPairedMaps reports whether the record contains both .map and .bsp files.
func (r *QuaddictedRecord) HasPairedMaps() bool {
	hasMap, hasBSP := false, false
	for fn := range r.Files {
		lower := strings.ToLower(fn)
		if strings.HasSuffix(lower, ".map") {
			hasMap = true
		}
		if strings.HasSuffix(lower, ".bsp") {
			hasBSP = true
		}
	}
	return hasMap && hasBSP
}

// FindQuaddictedDataDir locates or clones the quaddicted-data git repo.
func FindQuaddictedDataDir(explicit string) (string, error) {
	candidates := []string{
		explicit,
		".tmp/quaddicted-data",
		"dataset/bspdec/raw/quaddicted-data",
		"../quaddicted-data",
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			jsonDir := filepath.Join(c, "json", "by-sha256")
			if _, err := os.Stat(jsonDir); err == nil {
				return c, nil
			}
		}
	}

	cloneTarget := ".tmp/quaddicted-data"
	slog.Info("bspdec-corpus: cloning quaddicted-data metadata", "target", cloneTarget)
	cmd := exec.Command("git", "clone", "--depth", "1", "https://github.com/quaddicted/quaddicted-data.git", cloneTarget)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("clone quaddicted-data: %v: %s", err, out)
	}
	return cloneTarget, nil
}

// ScanQuaddictedCatalog walks quaddicted-data/json/by-sha256 and returns all records
// that contain both .map and .bsp files, sorted by package ID.
func ScanQuaddictedCatalog(quaddictedDataDir string) ([]QuaddictedRecord, error) {
	jsonDir := filepath.Join(quaddictedDataDir, "json", "by-sha256")
	var records []QuaddictedRecord

	err := filepath.WalkDir(jsonDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var rec QuaddictedRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			return nil
		}
		if rec.HasPairedMaps() {
			records = append(records, rec)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan quaddicted catalog: %w", err)
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].PkgID() < records[j].PkgID()
	})
	return records, nil
}

// FetchQuaddictedOptions configures catalog acquisition.
type FetchQuaddictedOptions struct {
	DataDir           string // dataset root (default dataset/bspdec)
	QuaddictedDataDir string // path to quaddicted-data metadata repo
	Limit             int    // maximum packages to fetch (0 = all)
	Workers           int    // parallel downloads (default 4)
	HTTPClient        *http.Client
}

// FetchQuaddictedPackages downloads and extracts paired .map/.bsp packages
// from Quaddicted and appends them to manifest.jsonl.
func FetchQuaddictedPackages(opts FetchQuaddictedOptions) error {
	if opts.DataDir == "" {
		opts.DataDir = "dataset/bspdec"
	}
	if opts.Workers <= 0 {
		opts.Workers = 4
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: 60 * time.Second}
	}

	qdDir, err := FindQuaddictedDataDir(opts.QuaddictedDataDir)
	if err != nil {
		return err
	}

	records, err := ScanQuaddictedCatalog(qdDir)
	if err != nil {
		return err
	}
	slog.Info("bspdec-corpus: discovered paired Quaddicted packages", "total", len(records))

	if opts.Limit > 0 && opts.Limit < len(records) {
		records = records[:opts.Limit]
	}

	rawDir := filepath.Join(opts.DataDir, "raw", "quaddicted")
	zipsDir := filepath.Join(rawDir, "zips")
	extractedDir := filepath.Join(rawDir, "extracted")
	if err := os.MkdirAll(zipsDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(extractedDir, 0o755); err != nil {
		return err
	}

	manifestPath := filepath.Join(opts.DataDir, "raw", "manifest.jsonl")
	var manifestEntries []eval.ManifestEntry
	var mu sync.Mutex

	recordChan := make(chan QuaddictedRecord, len(records))
	for _, r := range records {
		recordChan <- r
	}
	close(recordChan)

	var wg sync.WaitGroup
	var fetchErr error
	var errMu sync.Mutex

	for w := 0; w < opts.Workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for rec := range recordChan {
				pkgID := rec.PkgID()
				zipPath := filepath.Join(zipsDir, pkgID+".zip")

				// Download if missing or incomplete
				if err := downloadQuaddictedZip(opts.HTTPClient, rec, zipPath); err != nil {
					slog.Warn("bspdec-corpus: quaddicted download failed", "pkg", pkgID, "err", err)
					errMu.Lock()
					if fetchErr == nil {
						fetchErr = err
					}
					errMu.Unlock()
					continue
				}

				// Extract .map and .bsp
				targetPkgDir := filepath.Join(extractedDir, pkgID)
				maps, bsps, err := extractQuaddictedFiles(zipPath, targetPkgDir)
				if err != nil {
					slog.Warn("bspdec-corpus: quaddicted extract failed", "pkg", pkgID, "err", err)
					continue
				}
				if len(maps) == 0 || len(bsps) == 0 {
					continue
				}

				var relMaps, relBSPs []string
				for _, m := range maps {
					rel, _ := filepath.Rel(opts.DataDir, m)
					relMaps = append(relMaps, filepath.ToSlash(rel))
				}
				for _, b := range bsps {
					rel, _ := filepath.Rel(opts.DataDir, b)
					relBSPs = append(relBSPs, filepath.ToSlash(rel))
				}

				author := rec.TagValue("author")
				title := rec.TagValue("title")
				licenseNote := fmt.Sprintf("quaddicted: %s by %s", title, author)

				entry := eval.ManifestEntry{
					PkgID:       "quaddicted-" + pkgID,
					SourceURL:   rec.URLs[0],
					LicenseNote: licenseNote,
					MapFiles:    relMaps,
					BSPFiles:    relBSPs,
					Flags: eval.Flags{
						Era:            "classic",
						ToolchainGuess: "vanilla",
					},
				}

				mu.Lock()
				manifestEntries = append(manifestEntries, entry)
				mu.Unlock()
				slog.Info("bspdec-corpus: fetched and extracted", "pkg", pkgID, "maps", len(maps), "bsps", len(bsps))
			}
		}()
	}
	wg.Wait()

	if len(manifestEntries) > 0 {
		if err := eval.AppendToManifest(manifestPath, manifestEntries); err != nil {
			return fmt.Errorf("append manifest: %w", err)
		}
	}
	slog.Info("bspdec-corpus: quaddicted fetch complete", "entries", len(manifestEntries))
	return nil
}

func downloadQuaddictedZip(client *http.Client, rec QuaddictedRecord, destPath string) error {
	if fi, err := os.Stat(destPath); err == nil && fi.Size() > 0 {
		if rec.Bytes == 0 || fi.Size() == rec.Bytes {
			return nil // already downloaded
		}
	}

	var downloadURL string
	for _, u := range rec.URLs {
		if strings.Contains(u, "by-sha256") {
			downloadURL = u
			break
		}
	}
	if downloadURL == "" && len(rec.URLs) > 0 {
		downloadURL = rec.URLs[0]
	}
	if downloadURL == "" {
		return fmt.Errorf("no download URL in record")
	}

	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "ironwail-go-bspdec/1.0 (Quake research)")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, downloadURL)
	}

	tmpDest := destPath + ".part"
	f, err := os.Create(tmpDest)
	if err != nil {
		return err
	}

	hasher := sha256.New()
	w := io.MultiWriter(f, hasher)
	if _, err := io.Copy(w, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpDest)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpDest)
		return err
	}

	if rec.SHA256 != "" {
		actualHash := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(actualHash, rec.SHA256) {
			_ = os.Remove(tmpDest)
			return fmt.Errorf("sha256 mismatch: got %s, want %s", actualHash, rec.SHA256)
		}
	}

	return os.Rename(tmpDest, destPath)
}

func extractQuaddictedFiles(zipPath, targetDir string) (maps []string, bsps []string, err error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = zr.Close() }()

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return nil, nil, err
	}

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		baseName := filepath.Base(f.Name)
		lower := strings.ToLower(baseName)
		isMap := strings.HasSuffix(lower, ".map")
		isBSP := strings.HasSuffix(lower, ".bsp")
		isDoc := strings.HasSuffix(lower, ".txt") || strings.HasPrefix(lower, "readme") || strings.HasPrefix(lower, "license")

		if !isMap && !isBSP && !isDoc {
			continue
		}

		destPath := filepath.Join(targetDir, baseName)
		rc, err := f.Open()
		if err != nil {
			return nil, nil, err
		}
		out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			_ = rc.Close()
			return nil, nil, err
		}
		_, copyErr := io.Copy(out, rc)
		_ = out.Close()
		_ = rc.Close()
		if copyErr != nil {
			return nil, nil, copyErr
		}

		if isMap {
			maps = append(maps, destPath)
		} else if isBSP {
			bsps = append(bsps, destPath)
		}
	}
	return maps, bsps, nil
}
