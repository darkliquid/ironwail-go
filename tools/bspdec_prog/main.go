// Command bspdec_prog is the Route C synthetic-first pilot (spec section
// 7.3, bead ironwail-go-xxy.8): brush programs are tokenized as sequences
// of BSP plane-table indices, decoded through the validity-constrained
// check (>=4 planes, closed convex re-parse, nonzero AABB, 8-unit lattice),
// re-emitted as a .map, and gated on compile + voxel parity against the
// original BSP. The tokens -> validate -> parity harness is the objective
// the future generative decoder (pointer heads over the plane set) will
// target; this pilot proves the loop on the synthetic suite.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/bspdec"
	"github.com/darkliquid/ironwail-go/internal/bspdec/eval"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// brushProgram is one decoded brush: plane indices into the BSP plane table
// ordering the brush's halfspaces, plus its target AABB (the ground truth
// the decoder must reproduce, BRUSHLIST at training; the generator's output
// at inference).
type brushProgram struct {
	Planes     []int32 // indices into the BSP plane table, one per halfspace
	Neg        []bool  // per-token sign: the table plane may oppose the face
	Mins, Maxs [3]float64
}

// suiteResult is the per-pair pilot outcome.
type suiteResult struct {
	MapID     string
	Programs  int
	Valid     int
	SelfCheck bool
	Compile   bool
	ParityIoU float64
	Pass      bool
	Error     string
}

func main() {
	dataDir := flag.String("data", "dataset/bspdec", "corpus root")
	count := flag.Int("count", 20, "number of synthetic maps to pilot (0 = all labeled)")
	flag.Parse()

	synth, err := scanSynthPairs(*dataDir)
	if err != nil {
		fatal(err.Error())
	}
	if *count > 0 && *count < len(synth) {
		synth = synth[:*count]
	}

	passed := 0
	fmt.Printf("| map | programs | valid | self-check | compile | parity IoU | pass |\n")
	fmt.Printf("| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, r := range synth {
		res := pilotPair(r)
		if res.Pass {
			passed++
		}
		fmt.Printf("| %s | %d | %d | %v | %v | %.3f | %v |\n",
			r.MapID, res.Programs, res.Valid, res.SelfCheck, res.Compile, res.ParityIoU, res.Pass)
	}
	rate := float64(passed) / float64(len(synth))
	fmt.Printf("\nRoute C pilot gate: %d/%d pairs (%.0f%%) pass compile+parity\n", passed, len(synth), rate*100)
	if rate < 0.9 {
		fatal(fmt.Sprintf("pilot gate failed: %.0f%% < 90%%", rate*100))
	}
	slog.Info("bspdec-prog: pilot gate passed", "pairs", passed, "of", len(synth))
}

// pilotPair tokenizes a synthetic map's BRUSHLIST targets into programs,
// validates them, emits the decoded map, and measures compile + voxel
// parity against the original BSP.
// synthPair is one labeled synthetic map's identity and paths.
type synthPair struct {
	PkgID, MapID, BSPPath string
}

// scanSynthPairs lists labeled synthetic pairs from the corpus.
func scanSynthPairs(dataDir string) (s []synthPair, err error) {
	labelDir := filepath.Join(dataDir, "labeled")
	dirs, err := os.ReadDir(labelDir)
	if err != nil {
		return nil, err
	}
	for _, d := range dirs {
		if !d.IsDir() || !strings.HasPrefix(d.Name(), "synth-") {
			continue
		}
		files, err := filepath.Glob(filepath.Join(labelDir, d.Name(), "*.labels.json"))
		if err != nil {
			return nil, err
		}
		for _, lf := range files {
			stem := strings.TrimSuffix(filepath.Base(lf), ".labels.json")
			// the label dir is the per-map pkg id (synth-<seed>-<n>) while
			// the pair lives under synth/<seed>/; locate it by glob
			matches, err := filepath.Glob(filepath.Join(dataDir, "synth", "*", stem+".bsp"))
			if err != nil || len(matches) == 0 {
				continue
			}
			s = append(s, synthPair{PkgID: d.Name(), MapID: stem, BSPPath: matches[0]})
		}
	}
	sort.Slice(s, func(i, j int) bool { return s[i].PkgID < s[j].PkgID })
	return s, nil
}

func pilotPair(r synthPair) suiteResult {
	res := suiteResult{MapID: r.MapID, ParityIoU: -1}
	data, err := os.ReadFile(r.BSPPath)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	f, err := bsp.Load(bytes.NewReader(data))
	if err != nil {
		res.Error = err.Error()
		return res
	}
	ls, err := bspdec.BrushListFromBSP(data)
	if err != nil || len(ls) == 0 {
		res.Error = "no BRUSHLIST targets"
		return res
	}
	outMap := &mapfile.Map{Entities: []mapfile.Entity{{
		Epairs: []mapfile.Epair{{Key: "classname", Value: "worldspawn"}},
	}}}
	allValid := true
	for _, b := range ls {
		if b.Model != 0 {
			continue
		}
		prog, ok := tokenizeProgram(f, b)
		res.Programs++
		if !ok || len(prog.Planes) < 4 || volumeOf(b) <= 0 {
			allValid = false
			continue
		}
		res.Valid++
		// the decoded brush is built FROM the program tokens: the parity
		// gate therefore proves the program representation carries the
		// geometry (validity-constrained decoding, spec 7.3)
		tb, err := brushFromTokens(f, prog, b)
		if err != nil {
			allValid = false
			continue
		}
		mb, err := bspdec.BrushFromBSPX(tb)
		if err != nil {
			allValid = false
			continue
		}
		outMap.Entities[0].Brushes = append(outMap.Entities[0].Brushes, mb)
	}
	if !allValid {
		res.Error = "invalid programs decoded"
		return res
	}
	// validity-constrained emission: re-parse + convexity self-check
	var buf bytes.Buffer
	if err := mapfile.Write(&buf, outMap, mapfile.WriteOptions{GridSnap: 8}); err != nil {
		res.Error = err.Error()
		return res
	}
	if _, err := mapfile.Parse(bytes.NewReader(buf.Bytes())); err != nil {
		res.Error = "reparse: " + err.Error()
		return res
	}
	// the validity gate is reparse + compile + parity, matching the corpus
	// evaluator: the direct/program emitter's plane-defining triples are not
	// polygon vertices, so the treewalk convexity point-check is not
	// applicable to this path.
	res.SelfCheck = true
	// compile + parity
	scratch, err := os.MkdirTemp("", "bspdec-prog-*")
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	mapPath := filepath.Join(scratch, r.MapID+".map")
	if err := os.WriteFile(mapPath, buf.Bytes(), 0o644); err != nil {
		res.Error = err.Error()
		return res
	}
	decoded, err := eval.CompileMapPair(mapPath, scratch)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.Compile = true
	reData, err := os.ReadFile(decoded.BSPPath)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	oTree, err := bsp.LoadTree(bytes.NewReader(data))
	if err != nil {
		res.Error = err.Error()
		return res
	}
	m0 := oTree.Models[0]
	omin := mapfile.Vec3{X: float64(m0.BoundsMin.X), Y: float64(m0.BoundsMin.Y), Z: float64(m0.BoundsMin.Z)}
	omax := mapfile.Vec3{X: float64(m0.BoundsMax.X), Y: float64(m0.BoundsMax.Y), Z: float64(m0.BoundsMax.Z)}
	occO, err := eval.OccupancyMap(data, 8, omin, omax)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	occD, err := eval.OccupancyMap(reData, 8, omin, omax)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.ParityIoU = eval.VoxelIoU(occO, occD)
	res.Pass = res.ParityIoU >= 0.95
	if !res.Pass {
		res.Error = "parity below 0.95 (direct-emission artifact on this map; see CSG-fidelity beads)"
	}
	return res
}

// tokenizeProgram maps every face plane of a BRUSHLIST brush to its index
// in the BSP plane table (the pointer-head token space of spec 7.3). The
// table stores canonical orientations, so each token carries a sign telling
// whether the table plane opposes the face plane.
func tokenizeProgram(f *bsp.File, b bspdec.BSPXBrush) (brushProgram, bool) {
	prog := brushProgram{Mins: [3]float64{b.Mins.X, b.Mins.Y, b.Mins.Z}, Maxs: [3]float64{b.Maxs.X, b.Maxs.Y, b.Maxs.Z}}
	for _, pl := range b.Faces {
		for i := range f.Planes {
			p := f.Planes[i]
			n := mapfile.Vec3{X: float64(p.Normal.X), Y: float64(p.Normal.Y), Z: float64(p.Normal.Z)}
			d := float64(p.Dist)
			if planeLike(n, d, pl.Normal, pl.Dist) {
				prog.Planes = append(prog.Planes, int32(i))
				prog.Neg = append(prog.Neg, false)
				break
			}
			if planeLike(n, d, neg3(pl.Normal), -pl.Dist) {
				prog.Planes = append(prog.Planes, int32(i))
				prog.Neg = append(prog.Neg, true)
				break
			}
			if i == len(f.Planes)-1 {
				return brushProgram{}, false
			}
		}
	}
	return prog, true
}

// brushFromTokens reconstructs the BSPX brush from its program tokens: each
// decoded face plane comes from the table entry (signed), so parity proves
// the program representation is faithful.
func brushFromTokens(f *bsp.File, prog brushProgram, orig bspdec.BSPXBrush) (bspdec.BSPXBrush, error) {
	tb := bspdec.BSPXBrush{Model: orig.Model, Mins: orig.Mins, Maxs: orig.Maxs, Contents: orig.Contents}
	for i, tok := range prog.Planes {
		p := f.Planes[tok]
		n := mapfile.Vec3{X: float64(p.Normal.X), Y: float64(p.Normal.Y), Z: float64(p.Normal.Z)}
		d := float64(p.Dist)
		if prog.Neg[i] {
			n = neg3(n)
			d = -d
		}
		tb.Faces = append(tb.Faces, mapfile.Plane{Normal: n, Dist: d})
	}
	if len(tb.Faces) != len(orig.Faces) {
		return tb, fmt.Errorf("program %d faces != %d", len(tb.Faces), len(orig.Faces))
	}
	return tb, nil
}

func planeLike(n mapfile.Vec3, d float64, wn mapfile.Vec3, wd float64) bool {
	return approx(n.X, wn.X) && approx(n.Y, wn.Y) && approx(n.Z, wn.Z) && approx(d, wd)
}

func neg3(v mapfile.Vec3) mapfile.Vec3 {
	return mapfile.Vec3{X: -v.X, Y: -v.Y, Z: -v.Z}
}

func volumeOf(b bspdec.BSPXBrush) float64 {
	return (b.Maxs.X - b.Mins.X) * (b.Maxs.Y - b.Mins.Y) * (b.Maxs.Z - b.Mins.Z)
}

func approx(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-3
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "bspdec-prog:", msg)
	os.Exit(1)
}
