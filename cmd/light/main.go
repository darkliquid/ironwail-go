// Command light bakes lightmaps for a compiled BSP (bead ironwail-go-t63,
// M4): it parses faces and light entities, computes direct point lighting
// with shadow traces against the BSP tree, writes the Lighting lump and
// each lit face's lightofs, and optionally writes a QLIT v1 colored .lit
// sidecar.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/light"
	"github.com/darkliquid/ironwail-go/internal/qbsp"
)

func main() {
	outPath := flag.String("o", "", "output .bsp path (default: overwrite input)")
	lit := flag.Bool("lit", false, "write a QLIT v1 colored .lit sidecar")
	sun := flag.Bool("sun", false, "enable sun entity / sunlight worldspawn lighting")
	bounce := flag.Int("bounce", 0, "radiosity bounce count (0 = direct only)")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: light [-o out.bsp] [-lit] map.bsp")
		os.Exit(2)
	}
	bspPath := flag.Arg(0)

	bspData, err := os.ReadFile(bspPath)
	if err != nil {
		log.Fatalf("light: read %s: %v", bspPath, err)
	}

	faces, err := light.ParseFaces(bspData)
	if err != nil {
		log.Fatalf("light: parse faces: %v", err)
	}
	lights, err := light.ParseLights(bspData)
	if err != nil {
		log.Fatalf("light: parse lights: %v", err)
	}
	tree, err := bsp.LoadTree(bytes.NewReader(bspData))
	if err != nil {
		log.Fatalf("light: load tree: %v", err)
	}

	opts := light.BakeOpts{Bounce: *bounce}
	if *sun {
		s, err := light.ParseSun(bspData)
		if err != nil {
			log.Fatalf("light: parse sun: %v", err)
		}
		opts.Sun = s
	}
	res := light.BakeWithOpts(faces, lights, light.TreeTracer(tree), opts)
	if len(res.Lighting) == 0 {
		fmt.Println("light: no lightable faces (add light entities)")
	}

	outData, err := patchBSP(bspData, res)
	if err != nil {
		log.Fatalf("light: patch bsp: %v", err)
	}
	out := *outPath
	if out == "" {
		out = bspPath
	}
	if err := os.WriteFile(out, outData, 0o644); err != nil {
		log.Fatalf("light: write %s: %v", out, err)
	}
	fmt.Printf("light: wrote %d lightmap samples (%d lit faces) to %s\n", len(res.Lighting), countLit(res.LightOfs), out)

	if *lit {
		litPath := strings.TrimSuffix(out, filepath.Ext(out)) + ".lit"
		if err := os.WriteFile(litPath, light.WriteLit(&res), 0o644); err != nil {
			log.Fatalf("light: write %s: %v", litPath, err)
		}
		fmt.Printf("light: wrote %s\n", litPath)
	}
}

func countLit(ofs []int32) int {
	n := 0
	for _, o := range ofs {
		if o >= 0 {
			n++
		}
	}
	return n
}

// patchBSP sets each lit face's lightofs and replaces the lighting lump.
func patchBSP(bspData []byte, res light.Result) ([]byte, error) {
	version, lumps, err := qbsp.ReadBSPLumps(bytes.NewReader(bspData))
	if err != nil {
		return nil, err
	}
	// Faces lump (BSP29): 20 bytes/face, styles[4] at offset 12,
	// lightofs int32 at offset 16.
	facesLump := append([]byte(nil), lumps[7]...)
	for i := range res.Styles {
		off := i * 20
		if off+20 > len(facesLump) {
			continue
		}
		copy(facesLump[off+12:off+16], res.Styles[i][:])
	}
	for i, ofs := range res.LightOfs {
		if ofs < 0 {
			continue
		}
		off := i*20 + 16
		if off+4 > len(facesLump) {
			continue
		}
		binary.LittleEndian.PutUint32(facesLump[off:], uint32(ofs))
	}
	lumps[7] = facesLump
	lumps[8] = res.Lighting
	return qbsp.WriteBSP(lumps, version)
}
