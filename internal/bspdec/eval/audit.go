package eval

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/darkliquid/ironwail-go/internal/bspdec"
	"github.com/darkliquid/ironwail-go/internal/qbsp"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// ForwardVerdict categorizes forward compilation results.
type ForwardVerdict string

const (
	ForwardClean     ForwardVerdict = "CLEAN"
	ForwardCategoryA ForwardVerdict = "CATEGORY_A" // Inherent geometry void (leaks in ref compiler)
	ForwardCategoryB ForwardVerdict = "CATEGORY_B" // Compiler disparity (leaks in Go qbsp, sealed in ref compiler)
	ForwardFailed    ForwardVerdict = "FAILED"     // Parse or compiler error
	ForwardSkipped   ForwardVerdict = "SKIPPED"
)

// DecompVerdict categorizes decompilation/recompilation results.
type DecompVerdict string

const (
	DecompClean            DecompVerdict = "CLEAN"
	DecompCategoryCBoth    DecompVerdict = "CATEGORY_C_BOTH"    // Leaks in both Go qbsp and ref compiler
	DecompCategoryCGoOnly  DecompVerdict = "CATEGORY_C_GO_ONLY" // Leaks in Go qbsp, sealed in ref compiler
	DecompCategoryCRefOnly DecompVerdict = "CATEGORY_C_REF_ONLY" // Leaks in ref compiler, sealed in Go qbsp
	DecompFailed           DecompVerdict = "FAILED"
	DecompSkipped          DecompVerdict = "SKIPPED"
)

// ForwardCompileStats records the forward compilation analysis.
type ForwardCompileStats struct {
	GoLeaked     bool           `json:"go_leaked"`
	GoTrailLen   int            `json:"go_trail_len,omitempty"`
	RefAvailable bool           `json:"ref_available"`
	RefLeaked    bool           `json:"ref_leaked"`
	Verdict      ForwardVerdict `json:"verdict"`
	Error        string         `json:"error,omitempty"`
}

// DecompCompileStats records the decompilation and recompilation analysis.
type DecompCompileStats struct {
	DecompileError string        `json:"decompile_error,omitempty"`
	SelfCheckError string        `json:"self_check_error,omitempty"`
	GoLeaked       bool          `json:"go_leaked"`
	GoTrailLen     int           `json:"go_trail_len,omitempty"`
	RefAvailable   bool          `json:"ref_available"`
	RefLeaked      bool          `json:"ref_leaked"`
	Verdict        DecompVerdict `json:"verdict"`
	Error          string        `json:"error,omitempty"`
}

// AuditResult aggregates forward and decompilation audit findings for one map pair.
type AuditResult struct {
	PkgID   string              `json:"pkg_id"`
	MapID   string              `json:"map_id"`
	MapPath string              `json:"map_path,omitempty"`
	BSPPath string              `json:"bsp_path,omitempty"`
	Forward ForwardCompileStats `json:"forward"`
	Decomp  DecompCompileStats  `json:"decomp"`
}

// AuditOptions controls audit execution flags.
type AuditOptions struct {
	SkipForward bool
	SkipDecomp  bool
	GridSnap    int // 0 = off, default 8
}

// AuditMapPair runs forward and decompilation void checks on a single map pair.
func AuditMapPair(mapPath, bspPath, pkgID, mapID string, opts AuditOptions) (AuditResult, error) {
	res := AuditResult{
		PkgID:   pkgID,
		MapID:   mapID,
		MapPath: mapPath,
		BSPPath: bspPath,
		Forward: ForwardCompileStats{Verdict: ForwardSkipped},
		Decomp:  DecompCompileStats{Verdict: DecompSkipped},
	}

	snap := opts.GridSnap
	if snap <= 0 && !opts.SkipDecomp {
		snap = 8
	}

	// 1. Forward compile audit
	if mapPath != "" && !opts.SkipForward {
		fwdStats := ForwardCompileStats{}
		goLeaked, trailLen, goErr := compileMapWithGoQBSP(mapPath)
		if goErr != nil {
			fwdStats.Verdict = ForwardFailed
			fwdStats.Error = goErr.Error()
		} else {
			fwdStats.GoLeaked = goLeaked
			fwdStats.GoTrailLen = trailLen

			ref, refErr := CheckEricwToolsLeak(mapPath)
			if refErr == nil && ref.Available {
				fwdStats.RefAvailable = true
				fwdStats.RefLeaked = ref.Leaked
			}

			if !fwdStats.GoLeaked && (!fwdStats.RefAvailable || !fwdStats.RefLeaked) {
				fwdStats.Verdict = ForwardClean
			} else if fwdStats.RefAvailable && fwdStats.RefLeaked {
				fwdStats.Verdict = ForwardCategoryA
			} else if fwdStats.GoLeaked && fwdStats.RefAvailable && !fwdStats.RefLeaked {
				fwdStats.Verdict = ForwardCategoryB
			} else if fwdStats.GoLeaked {
				fwdStats.Verdict = ForwardCategoryB
			}
		}
		res.Forward = fwdStats
	}

	// 2. Decompilation & recompilation audit
	if bspPath != "" && !opts.SkipDecomp {
		decompStats := DecompCompileStats{}
		bspData, err := os.ReadFile(bspPath)
		if err != nil {
			decompStats.Verdict = DecompFailed
			decompStats.Error = fmt.Sprintf("read bsp: %v", err)
			res.Decomp = decompStats
			return res, nil
		}

		outMap, _, decErr := bspdec.Decompile(bspData, bspdec.Options{
			MergeConvex:     true,
			GridSnap:        snap,
			TextureFallback: "nearest",
		})
		if decErr != nil {
			decompStats.Verdict = DecompFailed
			decompStats.DecompileError = decErr.Error()
			decompStats.Error = fmt.Sprintf("decompile: %v", decErr)
			res.Decomp = decompStats
			return res, nil
		}

		if scErr := bspdec.SelfCheck(outMap); scErr != nil {
			decompStats.SelfCheckError = scErr.Error()
		}

		var buf bytes.Buffer
		if writeErr := mapfile.Write(&buf, outMap, mapfile.WriteOptions{GridSnap: snap}); writeErr != nil {
			decompStats.Verdict = DecompFailed
			decompStats.Error = fmt.Sprintf("write decomp map: %v", writeErr)
			res.Decomp = decompStats
			return res, nil
		}

		tmpDir, err := os.MkdirTemp("", "bspdec-decomp-audit-*")
		if err != nil {
			decompStats.Verdict = DecompFailed
			decompStats.Error = err.Error()
			res.Decomp = decompStats
			return res, nil
		}
		defer func() { _ = os.RemoveAll(tmpDir) }()

		tmpMap := filepath.Join(tmpDir, mapID+".decomp.map")
		if err := os.WriteFile(tmpMap, buf.Bytes(), 0o644); err != nil {
			decompStats.Verdict = DecompFailed
			decompStats.Error = err.Error()
			res.Decomp = decompStats
			return res, nil
		}

		goLeaked, trailLen, goErr := compileMapWithGoQBSP(tmpMap)
		if goErr != nil {
			decompStats.Verdict = DecompFailed
			decompStats.Error = fmt.Sprintf("recompile Go qbsp: %v", goErr)
			res.Decomp = decompStats
			return res, nil
		}
		decompStats.GoLeaked = goLeaked
		decompStats.GoTrailLen = trailLen

		ref, refErr := CheckEricwToolsLeak(tmpMap)
		if refErr == nil && ref.Available {
			decompStats.RefAvailable = true
			decompStats.RefLeaked = ref.Leaked
		}

		if !decompStats.GoLeaked && (!decompStats.RefAvailable || !decompStats.RefLeaked) {
			decompStats.Verdict = DecompClean
		} else if decompStats.GoLeaked && decompStats.RefAvailable && decompStats.RefLeaked {
			decompStats.Verdict = DecompCategoryCBoth
		} else if decompStats.GoLeaked && decompStats.RefAvailable && !decompStats.RefLeaked {
			decompStats.Verdict = DecompCategoryCGoOnly
		} else if !decompStats.GoLeaked && decompStats.RefAvailable && decompStats.RefLeaked {
			decompStats.Verdict = DecompCategoryCRefOnly
		} else if decompStats.GoLeaked {
			decompStats.Verdict = DecompCategoryCGoOnly
		}

		res.Decomp = decompStats
	}

	return res, nil
}

func compileMapWithGoQBSP(mapPath string) (leaked bool, trailLen int, err error) {
	data, err := os.ReadFile(mapPath)
	if err != nil {
		return false, 0, err
	}
	m, err := mapfile.Parse(bytes.NewReader(data))
	if err != nil {
		return false, 0, fmt.Errorf("parse: %w", err)
	}
	res, err := qbsp.Compile(m, qbsp.Options{})
	if err != nil {
		return false, 0, fmt.Errorf("compile: %w", err)
	}
	return res.Leaked, len(res.LeakPath), nil
}
