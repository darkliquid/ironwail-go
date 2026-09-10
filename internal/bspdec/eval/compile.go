package eval

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/qbsp"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// Pair is a canonicalized map/bsp pair under dataset/bspdec/paired/.
type Pair struct {
	MapPath string
	BSPPath string
}

// CompileMapPair compiles a .map with the pinned in-repo qbsp (which always
// appends the BRUSHLIST BSPX oracle) and writes <stem>.bsp plus <stem>.qbsp.log
// into outDir. Leaked compiles cross-check against ericw-tools (if available)
// and return an error carrying the leak trail length and comparison verdict.
func CompileMapPair(mapPath, outDir string) (Pair, error) {
	data, err := os.ReadFile(mapPath)
	if err != nil {
		return Pair{}, err
	}
	m, err := mapfile.Parse(bytes.NewReader(data))
	if err != nil {
		return Pair{}, fmt.Errorf("parse %s: %w", mapPath, err)
	}
	var logLines []string
	res, err := qbsp.Compile(m, qbsp.Options{
		Log: func(format string, a ...any) { logLines = append(logLines, fmt.Sprintf(format, a...)) },
	})
	if err != nil {
		return Pair{}, fmt.Errorf("compile %s: %w", mapPath, err)
	}
	if res.Leaked {
		ref, refErr := CheckEricwToolsLeak(mapPath)
		if refErr == nil && ref.Available {
			if ref.Leaked {
				return Pair{}, fmt.Errorf("compile %s: leaks to the void (trail %d points) [ericw-tools: verified void leak in geometry]", mapPath, len(res.LeakPath))
			}
			return Pair{}, fmt.Errorf("compile %s: leaks to the void (trail %d points) [ericw-tools: compiled sealed, Go qbsp disparity]", mapPath, len(res.LeakPath))
		}
		return Pair{}, fmt.Errorf("compile %s: leaks to the void (trail %d points)", mapPath, len(res.LeakPath))
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return Pair{}, err
	}
	stem := strings.TrimSuffix(filepath.Base(mapPath), filepath.Ext(mapPath))
	bspPath := filepath.Join(outDir, stem+".bsp")
	if err := os.WriteFile(bspPath, res.Data, 0o644); err != nil {
		return Pair{}, err
	}
	logPath := filepath.Join(outDir, stem+".qbsp.log")
	if err := os.WriteFile(logPath, []byte(strings.Join(logLines, "\n")+"\n"), 0o644); err != nil {
		return Pair{}, err
	}
	return Pair{MapPath: mapPath, BSPPath: bspPath}, nil
}

// BrushCounts reports per-model BRUSHLIST brush counts for a BSP (the oracle
// our compiler always appends); an empty result means no BRUSHLIST lump.
func BrushCounts(bspPath string) ([]int, error) {
	data, err := os.ReadFile(bspPath)
	if err != nil {
		return nil, err
	}
	counts, err := qbsp.ReadBSPXBrushList(data)
	if err != nil {
		if !bytes.Contains(data, []byte("BSPX")) {
			return nil, nil // no BRUSHLIST lump on disk
		}
		return nil, err
	}
	return counts, nil
}

// FindEricwQBSP looks for the ericw-tools reference compiler binary.
func FindEricwQBSP() string {
	if p := os.Getenv("ERICW_QBSP"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	dir, err := os.Getwd()
	if err == nil {
		for i := 0; i < 6; i++ {
			candidate := filepath.Join(dir, "bin", "ericw-tools", "qbsp")
			if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
				return candidate
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	if p, err := exec.LookPath("ericw-qbsp"); err == nil {
		return p
	}
	if p, err := exec.LookPath("qbsp-ericw"); err == nil {
		return p
	}
	return ""
}

// ReferenceLeakCheckResult reports the verdict of running the ericw-tools reference compiler.
type ReferenceLeakCheckResult struct {
	Available bool
	Leaked    bool
	Output    string
}

// CheckEricwToolsLeak runs the reference ericw-tools qbsp on a .map file (if available)
// to verify whether a leak warning corresponds to an open void in the map geometry
// vs a disparity in the Go qbsp compiler.
func CheckEricwToolsLeak(mapPath string) (ReferenceLeakCheckResult, error) {
	qbspPath := FindEricwQBSP()
	if qbspPath == "" {
		return ReferenceLeakCheckResult{Available: false}, nil
	}
	tmpDir, err := os.MkdirTemp("", "ericw-qbsp-check-*")
	if err != nil {
		return ReferenceLeakCheckResult{Available: true}, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	data, err := os.ReadFile(mapPath)
	if err != nil {
		return ReferenceLeakCheckResult{Available: true}, err
	}
	tmpMap := filepath.Join(tmpDir, filepath.Base(mapPath))
	if err := os.WriteFile(tmpMap, data, 0o644); err != nil {
		return ReferenceLeakCheckResult{Available: true}, err
	}
	outBSP := filepath.Join(tmpDir, "out.bsp")
	cmd := exec.Command(qbspPath, tmpMap, outBSP)
	out, _ := cmd.CombinedOutput()
	outStr := string(out)

	leaked := strings.Contains(outStr, "no filling performed") ||
		strings.Contains(outStr, "Reached occupant") ||
		strings.Contains(outStr, "Leak file written to")

	return ReferenceLeakCheckResult{
		Available: true,
		Leaked:    leaked,
		Output:    outStr,
	}, nil
}