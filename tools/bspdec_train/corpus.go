package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/bspdec"
	"github.com/darkliquid/ironwail-go/internal/bspdec/eval"
)

// labelRecord mirrors the corpus label records written by
// tools/bspdec_corpus (the bspdec.CellLabel/SeamLabel JSON shapes), so
// bspdec_train can read them without importing the corpus tool.
type labelRecord struct {
	MapID string             `json:"map_id"`
	Cells []bspdec.CellLabel `json:"cells"`
	Seams []bspdec.SeamLabel `json:"seams"`
	Stats map[string]int     `json:"stats"`
}

// mapRecord is one labeled map with its package-level split assignment
// (spec section 10: cacheable steps keyed by dataset SHA + config hash).
type mapRecord struct {
	PkgID  string
	MapID  string
	Split  string // train | val | test | holdout
	Labels *labelRecord
}

// scanCorpus sweeps the labeled slice of a corpus: manifest entries with
// sibling labels.json records, tagged with the package-level split from
// splits.json. It also returns the dataset SHA: sha256 over the manifest
// bytes and every label record, in sorted path order — the cache key for
// every training step.
func scanCorpus(dataDir string) ([]mapRecord, string, error) {
	var recs []mapRecord
	splits, err := loadSplits(filepath.Join(dataDir, "splits.json"))
	if err != nil {
		return nil, "", err
	}
	entries, err := eval.LoadManifest(filepath.Join(dataDir, "raw", "manifest.jsonl"))
	if err != nil {
		return nil, "", err
	}

	var hashInput []byte
	labeledDir := filepath.Join(dataDir, "labeled")
	var labelPaths []string
	for _, e := range entries {
		for _, mf := range e.MapFiles {
			stem := strings.TrimSuffix(filepath.Base(mf), filepath.Ext(mf))
			lp := filepath.Join(labeledDir, e.PkgID, stem+".labels.json")
			if _, err := os.Stat(lp); err != nil {
				continue
			}
			labelPaths = append(labelPaths, lp)
		}
	}
	// deterministic hash order across filesystems
	sort.Strings(labelPaths)
	for _, lp := range labelPaths {
		data, err := os.ReadFile(lp)
		if err != nil {
			return nil, "", err
		}
		hashInput = append(hashInput, data...)
		hashInput = append(hashInput, 0)
	}
	hashInput = append(hashInput, readManifestBytesOrEmpty(filepath.Join(dataDir, "raw", "manifest.jsonl"))...)
	sum := sha256.Sum256(hashInput)
	datasetSHA := hex.EncodeToString(sum[:])

	for _, e := range entries {
		split := splits.of(e.PkgID)
		for _, mf := range e.MapFiles {
			stem := strings.TrimSuffix(filepath.Base(mf), filepath.Ext(mf))
			lp := filepath.Join(labeledDir, e.PkgID, stem+".labels.json")
			data, err := os.ReadFile(lp)
			if err != nil {
				continue
			}
			var rec labelRecord
			if err := json.Unmarshal(data, &rec); err != nil {
				return nil, "", fmt.Errorf("labels %s: %w", lp, err)
			}
			recs = append(recs, mapRecord{PkgID: e.PkgID, MapID: stem, Split: split, Labels: &rec})
		}
	}
	sort.Slice(recs, func(i, j int) bool {
		if recs[i].PkgID != recs[j].PkgID {
			return recs[i].PkgID < recs[j].PkgID
		}
		return recs[i].MapID < recs[j].MapID
	})
	return recs, datasetSHA, nil
}

func readManifestBytesOrEmpty(p string) []byte {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	return b
}

// splitSet is the splits.json shape, with a pkgID -> split lookup map.
type splitSet struct {
	Train          []string `json:"train"`
	Val            []string `json:"val"`
	Test           []string `json:"test"`
	ClassicHoldout []string `json:"classic_holdout"`
	m              map[string]string
}

func loadSplits(path string) (*splitSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s splitSet
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	set := map[string]string{}
	for _, p := range s.ClassicHoldout {
		set[p] = "holdout"
	}
	for _, p := range s.Test {
		set[p] = "test"
	}
	for _, p := range s.Val {
		set[p] = "val"
	}
	for _, p := range s.Train {
		set[p] = "train"
	}
	s.m = set
	return &s, nil
}

func (s *splitSet) of(pkgID string) string {
	if v, ok := s.m[pkgID]; ok {
		return v
	}
	return "holdout"
}

// configHash keys the training configuration: route, seed, and the feature
// schema version any future model format changes must bump. Combined with
// the dataset SHA it forms the step-cache key (spec section 10).
func configHash(route string, seed int64, featureSchema string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("route=%s|seed=%d|schema=%s", route, seed, featureSchema)))
	return hex.EncodeToString(sum[:])
}
