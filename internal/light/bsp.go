package light

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/qbsp"
)

// ParseFaces decodes the BSP's face geometry (BSP29) into lightable faces:
// each face's polygon (from vertexes/edges/surfedges), texinfo vectors,
// and plane normal. Sky faces are marked; TEX_SPECIAL faces are skipped.
func ParseFaces(bspData []byte) ([]Face, error) {
	_, lumps, err := qbsp.ReadBSPLumps(bytes.NewReader(bspData))
	if err != nil {
		return nil, err
	}
	planes := parsePlanes(lumps[1])
	vertexes := parseVertexes(lumps[3])
	texinfos := parseTexinfos(lumps[6])
	faces := parseFacesLump(lumps[7])
	edges := parseEdges(lumps[12])
	surfedges := parseSurfedges(lumps[13])
	textures := lumps[2]

	out := make([]Face, 0, len(faces))
	for fi, f := range faces {
		if f.Texinfo < 0 || int(f.Texinfo) >= len(texinfos) {
			continue
		}
		ti := texinfos[f.Texinfo]
		if ti.Miptex < 0 || int(ti.Miptex) >= numTextures(textures) {
			continue
		}
		name := textureName(textures, int(ti.Miptex))
		sky := strings.HasPrefix(name, "sky")
		noDraw := ti.Flags&bsp.TexSpecial != 0

		var poly [][3]float64
		ok := true
		for e := 0; e < int(f.NumEdges); e++ {
			se := surfedges[int(f.FirstEdge)+e]
			ei := int(se)
			reverse := false
			if ei < 0 {
				ei = -ei - 1
				reverse = true
			}
			if ei < 0 || ei >= len(edges) {
				ok = false
				break
			}
			ed := edges[ei]
			vi := ed[0]
			if reverse {
				vi = ed[1]
			}
			if vi < 0 || vi >= len(vertexes) {
				ok = false
				break
			}
			poly = append(poly, vertexes[vi])
		}
		if !ok || len(poly) < 3 {
			continue
		}

		pl := planes[f.Planenum]
		normal := pl.Normal
		if f.Side != 0 {
			normal = [3]float64{-normal[0], -normal[1], -normal[2]}
		}

		out = append(out, Face{
			Index:  fi,
			Poly:   poly,
			Vecs:   ti.Vecs,
			Normal: normal,
			Sky:    sky,
			NoDraw: noDraw,
		})
	}
	return out, nil
}

type bspPlane struct {
	Normal [3]float64
	Dist   float64
}

type bspTexinfo struct {
	Vecs   [2][4]float64
	Miptex int32
	Flags  int32
}

type bspFace struct {
	Planenum  int32
	Side      int32
	FirstEdge int32
	NumEdges  int32
	Texinfo   int32
}

func parsePlanes(lump []byte) []bspPlane {
	var out []bspPlane
	for i := 0; i+20 <= len(lump); i += 20 {
		out = append(out, bspPlane{
			Normal: [3]float64{
				float64(math.Float32frombits(binary.LittleEndian.Uint32(lump[i:]))),
				float64(math.Float32frombits(binary.LittleEndian.Uint32(lump[i+4:]))),
				float64(math.Float32frombits(binary.LittleEndian.Uint32(lump[i+8:]))),
			},
			Dist: float64(math.Float32frombits(binary.LittleEndian.Uint32(lump[i+12:]))),
		})
	}
	return out
}

func parseVertexes(lump []byte) [][3]float64 {
	var out [][3]float64
	for i := 0; i+12 <= len(lump); i += 12 {
		out = append(out, [3]float64{
			float64(math.Float32frombits(binary.LittleEndian.Uint32(lump[i:]))),
			float64(math.Float32frombits(binary.LittleEndian.Uint32(lump[i+4:]))),
			float64(math.Float32frombits(binary.LittleEndian.Uint32(lump[i+8:]))),
		})
	}
	return out
}

func parseTexinfos(lump []byte) []bspTexinfo {
	var out []bspTexinfo
	for i := 0; i+40 <= len(lump); i += 40 {
		var ti bspTexinfo
		for j := 0; j < 2; j++ {
			for k := 0; k < 4; k++ {
				ti.Vecs[j][k] = float64(math.Float32frombits(binary.LittleEndian.Uint32(lump[i+j*16+k*4:])))
			}
		}
		ti.Miptex = int32(binary.LittleEndian.Uint32(lump[i+32:]))
		ti.Flags = int32(binary.LittleEndian.Uint32(lump[i+36:]))
		out = append(out, ti)
	}
	return out
}

func parseFacesLump(lump []byte) []bspFace {
	var out []bspFace
	for i := 0; i+20 <= len(lump); i += 20 {
		out = append(out, bspFace{
			Planenum:  int32(binary.LittleEndian.Uint16(lump[i:])),
			Side:      int32(binary.LittleEndian.Uint16(lump[i+2:])),
			FirstEdge: int32(binary.LittleEndian.Uint32(lump[i+4:])),
			NumEdges:  int32(binary.LittleEndian.Uint16(lump[i+8:])),
			Texinfo:   int32(binary.LittleEndian.Uint16(lump[i+10:])),
		})
	}
	return out
}

func parseEdges(lump []byte) [][2]int {
	var out [][2]int
	for i := 0; i+4 <= len(lump); i += 4 {
		out = append(out, [2]int{
			int(binary.LittleEndian.Uint16(lump[i:])),
			int(binary.LittleEndian.Uint16(lump[i+2:])),
		})
	}
	return out
}

func parseSurfedges(lump []byte) []int32 {
	var out []int32
	for i := 0; i+4 <= len(lump); i += 4 {
		out = append(out, int32(binary.LittleEndian.Uint32(lump[i:])))
	}
	return out
}

func numTextures(lump []byte) int {
	if len(lump) < 4 {
		return 0
	}
	return int(binary.LittleEndian.Uint32(lump[0:]))
}

func textureName(lump []byte, idx int) string {
	if idx < 0 || idx >= numTextures(lump) || len(lump) < 4+4*(idx+1) {
		return ""
	}
	ofs := int(binary.LittleEndian.Uint32(lump[4+idx*4:]))
	if ofs < 0 || ofs+16 > len(lump) {
		return ""
	}
	name := lump[ofs : ofs+16]
	if i := bytes.IndexByte(name, 0); i >= 0 {
		name = name[:i]
	}
	return string(name)
}

// ParseLights extracts point light entities from the BSP's entity lump.
func ParseLights(bspData []byte) ([]Light, error) {
	_, lumps, err := qbsp.ReadBSPLumps(bytes.NewReader(bspData))
	if err != nil {
		return nil, err
	}
	var lights []Light
	for _, e := range parseEntities(lumps[0]) {
		if e["classname"] != "light" {
			continue
		}
		origin, ok := e["origin"]
		if !ok {
			continue
		}
		var o [3]float64
		if _, err := fmt.Sscanf(origin, "%f %f %f", &o[0], &o[1], &o[2]); err != nil {
			continue
		}
		value := 300.0
		if v, ok := e["light"]; ok {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				value = f
			}
		}
		lights = append(lights, Light{Origin: o, Value: value})
	}
	return lights, nil
}

// parseEntities parses the entity lump into per-entity key/value maps.
func parseEntities(lump []byte) []map[string]string {
	toks := tokenizeEntities(lump)
	var out []map[string]string
	var cur map[string]string
	i := 0
	for i < len(toks) {
		switch toks[i] {
		case "{":
			cur = map[string]string{}
			i++
		case "}":
			if len(cur) > 0 {
				out = append(out, cur)
			}
			cur = nil
			i++
		default:
			if cur == nil {
				i++
				continue
			}
			key := toks[i]
			i++
			val := ""
			if i < len(toks) && toks[i] != "{" && toks[i] != "}" {
				val = toks[i]
				i++
			}
			cur[key] = val
		}
	}
	return out
}

// tokenizeEntities splits the entity lump into tokens: braces, quoted
// strings, and bare words.
func tokenizeEntities(lump []byte) []string {
	var toks []string
	i := 0
	for i < len(lump) {
		c := lump[i]
		switch {
		case c == '{' || c == '}':
			toks = append(toks, string(c))
			i++
		case c == '"':
			j := i + 1
			for j < len(lump) && lump[j] != '"' {
				j++
			}
			toks = append(toks, string(lump[i+1:j]))
			i = j + 1
		case c <= ' ':
			i++
		default:
			j := i
			for j < len(lump) && lump[j] > ' ' {
				j++
			}
			toks = append(toks, string(lump[i:j]))
			i = j
		}
	}
	return toks
}
