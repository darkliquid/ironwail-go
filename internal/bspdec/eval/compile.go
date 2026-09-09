package eval

import (
	"bytes"
	"fmt"
	"os"
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
// into outDir. Leaked compiles return an error carrying the leak trail length.
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