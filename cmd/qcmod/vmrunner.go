package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

// vmWorld is the plan 25 Phase B In-VM runner: it executes REAL progs.dat
// bytecode through internal/qc.VM, mirroring how the engine dispatches QC
// callbacks (set self/other/time globals, call the function). This is the
// authoritative counterpart to sim.World (which runs QuakeGo *functions*
// natively In-Go); the mode-parity suite (qv) asserts the two agree.
//
// Unlike the full engine, vmWorld needs no server, no physics, no assets —
// just the VM, edict storage, and the global defs the engine writes.
type vmWorld struct {
	vm *qc.VM

	// registry of spawned entity numbers, mirroring sim.World.Ents-ish.
	spawned []int
}

// newVMWorld boots a VM from compiled progs bytes. When progs is nil, it
// compiles pkg/qgo/quakego deterministically (walking up from the working dir
// to find the repo root, then invoking cmd/qgo like the engine tests do).
func newVMWorld(progs []byte) (*vmWorld, error) {
	var err error
	if progs == nil {
		progs, err = compileProgs()
		if err != nil {
			return nil, fmt.Errorf("compile progs: %w", err)
		}
	}
	vm := qc.NewVM()
	if err := vm.LoadProgs(bytes.NewReader(progs)); err != nil {
		return nil, fmt.Errorf("load progs: %w", err)
	}
	qc.RegisterBuiltins(vm)
	// LoadProgs sets EntityFields/EdictSize from the progs header. Preallocate
	// a small edict table (world + 8 slots) so builtins that dereference
	// `self`/`other` as edicts resolve; tests/REPL spawn more on demand.
	vm.NumEdicts = 9 // world (0) + 8 editable slots
	vm.MaxEdicts = 64
	vm.EdictSize = vm.EntityFields*4 + 28
	vm.Edicts = make([]byte, vm.EdictSize*vm.MaxEdicts)
	// Zero the preallocated slots' private data, exactly as ED_ClearEdict
	// does: a mod function that reads self.Touch/Use/Think/className before
	// any spawn setup must see zeroes, not garbage (the engine relies on
	// this too — see server_runtime.go AllocEdict clearQCVMEdictData).
	for e := 1; e < vm.NumEdicts; e++ {
		data := vm.EdictData(e)
		for i := range data {
			data[i] = 0
		}
	}

	// Register the globals the engine writes into the VM (self/other/time).
	vm.GlobalDefs = append(vm.GlobalDefs,
		qc.DDef{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		qc.DDef{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		qc.DDef{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
		qc.DDef{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSParm0), Name: vm.AllocString("parm0")},
	)

	return &vmWorld{vm: vm}, nil
}

// fire calls a QC function by index with self (and optionally other) set and
// returns the number of edicts spawned during the call (matching
// SyncSpawnedEdictsFromQCVM semantics — newly spawned edicts need relinking).
func (w *vmWorld) fire(funcIdx int, self int, other int, time float32) (spawned int, err error) {
	prev := w.vm.NumEdicts
	w.vm.SetGInt(qc.OFSSelf, int32(self))
	w.vm.SetGInt(qc.OFSOther, int32(other))
	w.vm.SetGInt(qc.OFSTime, int32(math.Float32bits(time)))
	if err := w.vm.ExecuteFunction(funcIdx); err != nil {
		return 0, err
	}
	spawned = w.vm.NumEdicts - prev
	if spawned > 0 {
		w.spawned = append(w.spawned, prev+1) // record first new edict num
	}
	return spawned, nil
}

// fireByName is the ergonomic wrapper used by tests/REPL: resolve name, fire.
func (w *vmWorld) fireByName(name string, self, other int, time float32) error {
	idx := w.vm.FindFunction(name)
	if idx < 0 {
		return fmt.Errorf("qcmod: no function %q in progs", name)
	}
	_, err := w.fire(idx, self, other, time)
	return err
}

// compileProgs compiles pkg/qgo/quakego into progs.dat bytes by invoking
// cmd/qgo, walking up from the working directory to find the repo root.
// This is the qcmod equivalent of testutil.CompileProgsDataFromSource (which
// assumes a test working dir); qcmod runs from anywhere, so it resolves the
// root itself.
// progsTempPath is the fixed location compileProgsToTemp writes to, so
// callers can locate side-car artifacts without recompiling.
func progsTempPath() string {
	return filepath.Join(os.TempDir(), "ironwail-go-qcmod-progs.dat")
}

// compileProgsPath compiles the QuakeGo sources and returns the progs.dat
// path, so callers can find side-car artifacts (the .map source map) written
// next to it by qgo.
func compileProgsPath() (string, error) {
	_, path, err := compileProgsToTemp()
	return path, err
}

// compileProgsToTemp compiles pkg/qgo/quakego with cmd/qgo into a temporary
// progs.dat, returning its bytes and path. qgo also writes a side-car source
// map at <path>.map when source-map generation is enabled (the default).
func compileProgsToTemp() ([]byte, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}
	// Walk up to the repo root (where go.mod + pkg/qgo/quakego live).
	root := cwd
	for {
		if _, err := os.Stat(filepath.Join(root, "pkg", "qgo", "quakego", "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			return nil, "", fmt.Errorf("repo root not found above %s (need pkg/qgo/quakego)", cwd)
		}
		root = parent
	}
	progsSrc := filepath.Join(root, "pkg", "qgo", "quakego")
	out := progsTempPath()
	cmd := exec.Command("go", "run", filepath.Join(root, "cmd", "qgo"), "-o", out, ".")
	cmd.Dir = progsSrc
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, "", fmt.Errorf("qgo compile: %w: %s", err, stderr.String())
	}
	data, err := os.ReadFile(out)
	if err != nil {
		return nil, "", err
	}
	return data, out, nil
}

// loadProgsSourceMap reads the side-car source map for a progs.dat path.
// Returns nil (not an error) when no map was generated.
func loadProgsSourceMap(progsPath string) *qc.SourceMap {
	if progsPath == "" {
		return nil
	}
	f, err := os.Open(progsPath + ".map")
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	sm, err := qc.LoadSourceMap(f)
	if err != nil {
		return nil
	}
	return sm
}

func compileProgs() ([]byte, error) {
	data, _, err := compileProgsToTemp()
	if err != nil {
		return nil, err
	}
	return data, nil
}
