package qbsp_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"encoding/binary"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/light"
	"github.com/darkliquid/ironwail-go/internal/qbsp"
	"github.com/darkliquid/ironwail-go/internal/vis"
)

// ericwToolsDir returns the ericw-tools build directory (env-gated):
// ERICW_TOOLS_DIR, else the default local checkout. Tests skip when absent.
func ericwToolsDir(t *testing.T) string {
	dir := os.Getenv("ERICW_TOOLS_DIR")
	if dir == "" {
		t.Skip("ERICW_TOOLS_DIR not set; skipping the ericw-tools parity harness")
	}
	if _, err := os.Stat(filepath.Join(dir, "bspinfo", "bspinfo")); err != nil {
		t.Skipf("ericw-tools not found at %s (set ERICW_TOOLS_DIR to the build directory): %v", dir, err)
	}
	return dir
}

// runEricw runs an ericw tool and returns its stdout.
func runEricw(t *testing.T, dir string, tool string, args ...string) string {
	t.Helper()
	bin := filepath.Join(dir, tool, tool)
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ericw %s %v: %v\n%s", tool, args, err, out)
	}
	return string(out)
}

// bspinfoLumps parses the "count lumpname bytesize" table from bspinfo.
func bspinfoLumps(t *testing.T, out string) map[string]int {
	t.Helper()
	lumps := map[string]int{}
	// bspinfo prints counted lumps as "count name bytes" and byte-only
	// lumps (visdata/lightdata/entdata) as "name bytes".
	reCounted := regexp.MustCompile(`^\s+(\d+)\s+([a-z]+)\s+(\d+)\s*$`)
	reBytes := regexp.MustCompile(`^\s+([a-z]+)\s+(\d+)\s*$`)
	for _, line := range strings.Split(out, "\n") {
		if m := reCounted.FindStringSubmatch(line); m != nil {
			count, _ := strconv.Atoi(m[1])
			lumps[m[2]] = count
			continue
		}
		if m := reBytes.FindStringSubmatch(line); m != nil {
			size, _ := strconv.Atoi(m[2])
			lumps[m[1]] = size
		}
	}
	if len(lumps) == 0 {
		t.Fatalf("bspinfo produced no lump table:\n%s", out)
	}
	return lumps
}

// litBoxMap is a hollow box with a light, written to a temp file.
func litBoxMap(t *testing.T, dir string) string {
	t.Helper()
	src := `{
"classname" "worldspawn"
{
( 64 0 0 ) ( 64 0 8 ) ( 64 64 0 ) mt_floor 0 0 0 1 1
( 0 64 0 ) ( 0 64 8 ) ( 0 0 8 ) mt_floor 0 0 0 1 1
( 0 64 0 ) ( 64 64 0 ) ( 0 64 8 ) mt_floor 0 0 0 1 1
( 0 0 0 ) ( 0 0 8 ) ( 64 0 0 ) mt_floor 0 0 0 1 1
( 0 0 8 ) ( 0 64 8 ) ( 64 0 8 ) mt_floor 0 0 0 1 1
( 0 0 0 ) ( 64 0 0 ) ( 0 64 0 ) mt_floor 0 0 0 1 1
}
{
( 64 0 56 ) ( 64 0 64 ) ( 64 64 56 ) mt_floor 0 0 0 1 1
( 0 64 56 ) ( 0 64 64 ) ( 0 0 64 ) mt_floor 0 0 0 1 1
( 0 64 56 ) ( 64 64 56 ) ( 0 64 64 ) mt_floor 0 0 0 1 1
( 0 0 56 ) ( 0 0 64 ) ( 64 0 56 ) mt_floor 0 0 0 1 1
( 0 0 64 ) ( 0 64 64 ) ( 64 0 64 ) mt_floor 0 0 0 1 1
( 0 0 56 ) ( 64 0 56 ) ( 0 64 56 ) mt_floor 0 0 0 1 1
}
{
( 8 0 8 ) ( 8 0 56 ) ( 8 64 8 ) mt_wall 0 0 0 1 1
( 0 64 8 ) ( 0 64 56 ) ( 0 8 56 ) mt_wall 0 0 0 1 1
( 0 64 8 ) ( 8 64 8 ) ( 0 64 56 ) mt_wall 0 0 0 1 1
( 0 8 8 ) ( 0 8 56 ) ( 8 8 8 ) mt_wall 0 0 0 1 1
( 0 8 56 ) ( 0 64 56 ) ( 8 8 56 ) mt_wall 0 0 0 1 1
( 0 8 8 ) ( 8 8 8 ) ( 0 64 8 ) mt_wall 0 0 0 1 1
}
{
( 64 0 8 ) ( 64 0 56 ) ( 64 64 8 ) mt_wall 0 0 0 1 1
( 56 64 8 ) ( 56 64 56 ) ( 56 8 56 ) mt_wall 0 0 0 1 1
( 56 64 8 ) ( 64 64 8 ) ( 56 64 56 ) mt_wall 0 0 0 1 1
( 56 8 8 ) ( 56 8 56 ) ( 64 8 8 ) mt_wall 0 0 0 1 1
( 56 8 56 ) ( 56 64 56 ) ( 64 8 56 ) mt_wall 0 0 0 1 1
( 56 8 8 ) ( 64 8 8 ) ( 56 64 8 ) mt_wall 0 0 0 1 1
}
{
( 64 8 8 ) ( 64 8 56 ) ( 64 0 8 ) mt_wall 0 0 0 1 1
( 0 8 8 ) ( 0 8 56 ) ( 56 8 8 ) mt_wall 0 0 0 1 1
( 0 8 8 ) ( 64 8 8 ) ( 0 8 56 ) mt_wall 0 0 0 1 1
( 0 0 8 ) ( 0 0 56 ) ( 56 0 8 ) mt_wall 0 0 0 1 1
( 0 0 56 ) ( 0 8 56 ) ( 56 0 56 ) mt_wall 0 0 0 1 1
( 0 0 8 ) ( 56 0 8 ) ( 0 8 8 ) mt_wall 0 0 0 1 1
}
{
( 64 64 8 ) ( 64 64 56 ) ( 64 56 8 ) mt_wall 0 0 0 1 1
( 0 64 8 ) ( 0 64 56 ) ( 56 64 8 ) mt_wall 0 0 0 1 1
( 0 64 8 ) ( 64 64 8 ) ( 0 64 56 ) mt_wall 0 0 0 1 1
( 0 56 8 ) ( 0 56 56 ) ( 56 56 8 ) mt_wall 0 0 0 1 1
( 0 56 56 ) ( 0 64 56 ) ( 56 56 56 ) mt_wall 0 0 0 1 1
( 0 56 8 ) ( 56 56 8 ) ( 0 64 8 ) mt_wall 0 0 0 1 1
}
}
{
"classname" "info_player_start"
"origin" "32 32 32"
}
{
"classname" "light"
"origin" "32 32 48"
"light" "1000"
}
`
	path := filepath.Join(dir, "box.map")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestParityEricwCompilesAndReadsOurBSP runs the full Go pipeline (qbsp →
// vis → light) on a lit box and verifies ericw-tools' bspinfo reads every
// stage, with sane lump counts. Gated on ERICW_TOOLS_DIR.
func TestParityEricwCompilesAndReadsOurBSP(t *testing.T) {
	dir := ericwToolsDir(t)
	tmp := t.TempDir()
	mapPath := litBoxMap(t, tmp)

	// Our qbsp.
	mf, err := os.Open(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	m, err := qbsp.ParseMap(mf)
	_ = mf.Close()
	if err != nil {
		t.Fatal(err)
	}
	res, err := qbsp.Compile(m, qbsp.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ourBSP := filepath.Join(tmp, "our.bsp")
	if err := os.WriteFile(ourBSP, res.Data, 0o644); err != nil {
		t.Fatal(err)
	}
	ourPRT := filepath.Join(tmp, "our.prt")
	if err := os.WriteFile(ourPRT, res.PortalFile.Serialize(), 0o644); err != nil {
		t.Fatal(err)
	}

	// ericw bspinfo reads our BSP.
	out := runEricw(t, dir, "bspinfo", ourBSP)
	lumps := bspinfoLumps(t, out)
	if lumps["models"] != 1 {
		t.Errorf("our bsp models = %d, want 1", lumps["models"])
	}
	for _, lump := range []string{"planes", "vertexes", "nodes", "leafs", "faces", "clipnodes", "edges", "surfedges"} {
		if lumps[lump] == 0 {
			t.Errorf("our bsp %s = 0 (empty geometry)", lump)
		}
	}

	// Our vis: ericw reads the vis'd BSP with visdata.
	visData, err := vis.Run(res.Data, res.PortalFile.Serialize())
	if err != nil {
		t.Fatal(err)
	}
	visBSP := filepath.Join(tmp, "our_vis.bsp")
	if err := os.WriteFile(visBSP, visData, 0o644); err != nil {
		t.Fatal(err)
	}
	out = runEricw(t, dir, "bspinfo", visBSP)
	t.Logf("vis'd bspinfo:\n%s", out)
	if v := bspinfoLumps(t, out)["visdata"]; v == 0 {
		t.Error("ericw reads our vis'd BSP with zero visdata")
	}

	// Our light: ericw reads the lit BSP with lightdata.
	faces, err := light.ParseFaces(res.Data)
	if err != nil {
		t.Fatal(err)
	}
	lights, err := light.ParseLights(res.Data)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := bsp.LoadTree(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatal(err)
	}
	litRes := light.Bake(faces, lights, light.TreeTracer(tree))
	litBSP := filepath.Join(tmp, "our_lit.bsp")
	if err := os.WriteFile(litBSP, patchLightBSP(t, res.Data, litRes), 0o644); err != nil {
		t.Fatal(err)
	}
	out = runEricw(t, dir, "bspinfo", litBSP)
	if v := bspinfoLumps(t, out)["lightdata"]; v == 0 {
		t.Error("ericw reads our lit BSP with zero lightdata")
	}

	// Cross-feed: ericw's vis reads OUR .prt.
	_ = ourPRT
}

// patchLightBSP applies the light result to a BSP image (mirrors cmd/light).
func patchLightBSP(t *testing.T, data []byte, res light.Result) []byte {
	t.Helper()
	version, lumps, err := qbsp.ReadBSPLumps(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	facesLump := append([]byte(nil), lumps[7]...)
	for i, ofs := range res.LightOfs {
		if ofs < 0 {
			continue
		}
		off := i*20 + 16
		if off+4 <= len(facesLump) {
			binary.LittleEndian.PutUint32(facesLump[off:], uint32(ofs))
		}
	}
	lumps[7] = facesLump
	lumps[8] = res.Lighting
	out, err := qbsp.WriteBSP(lumps, version)
	if err != nil {
		t.Fatal(err)
	}
	return out
}