package bsp

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/darkliquid/ironwail-go/pkg/types"
)

// Portal represents one PRT1 portal: the shared facet polygon between two leaves.
type Portal struct {
	Leafs  [2]int
	Points []types.Vec3d
}

// PortalFile represents a complete PRT1 file defining the leaf-to-leaf portal topology.
type PortalFile struct {
	LeafCount int
	Portals   []Portal
}

// ParsePortalFile parses a PRT1 portal file from r.
func ParsePortalFile(r io.Reader) (*PortalFile, error) {
	tok := newPrtTokenReader(r)
	magic, ok := tok.Next()
	if !ok {
		return nil, fmt.Errorf("empty portal file")
	}
	if magic != "PRT1" && magic != "PRT1-AM" {
		return nil, fmt.Errorf("unknown portal file header %q", magic)
	}

	pf := &PortalFile{}
	lc, err := tok.Int()
	if err != nil {
		return nil, fmt.Errorf("read leaf count: %w", err)
	}
	num, err := tok.Int()
	if err != nil {
		return nil, fmt.Errorf("read portal count: %w", err)
	}
	pf.LeafCount = int(lc)

	for i := 0; i < int(num); i++ {
		np, err := tok.Int()
		if err != nil {
			return nil, fmt.Errorf("read portal %d point count: %w", i, err)
		}
		l0, err := tok.Int()
		if err != nil {
			return nil, fmt.Errorf("read portal %d leaf 0: %w", i, err)
		}
		l1, err := tok.Int()
		if err != nil {
			return nil, fmt.Errorf("read portal %d leaf 1: %w", i, err)
		}
		if int(l0) > pf.LeafCount || int(l1) > pf.LeafCount {
			return nil, fmt.Errorf("portal %d leaf out of bounds (%d, %d > %d)", i, l0, l1, pf.LeafCount)
		}

		p := Portal{Leafs: [2]int{int(l0), int(l1)}}
		for j := 0; j < int(np); j++ {
			// Skip optional "(" token
			nxt, _ := tok.Next()
			if nxt == "(" {
				nxt, _ = tok.Next()
			}
			x, err := strconv.ParseFloat(nxt, 64)
			if err != nil {
				return nil, fmt.Errorf("portal %d point %d x: %w", i, j, err)
			}
			yStr, _ := tok.Next()
			y, err := strconv.ParseFloat(yStr, 64)
			if err != nil {
				return nil, fmt.Errorf("portal %d point %d y: %w", i, j, err)
			}
			zStr, _ := tok.Next()
			z, err := strconv.ParseFloat(zStr, 64)
			if err != nil {
				return nil, fmt.Errorf("portal %d point %d z: %w", i, j, err)
			}
			// Consume optional ")"
			if nz, ok := tok.Peek(); ok && nz == ")" {
				_, _ = tok.Next()
			}
			p.Points = append(p.Points, types.Vec3d{X: x, Y: y, Z: z})
		}
		if len(p.Points) < 3 {
			return nil, fmt.Errorf("portal %d degenerate winding (%d points)", i, len(p.Points))
		}
		pf.Portals = append(pf.Portals, p)
	}

	return pf, nil
}

// Serialize renders the PRT1 text representation readable by vis tools.
func (pf *PortalFile) Serialize() []byte {
	if pf == nil || pf.LeafCount == 0 {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", "PRT1")
	fmt.Fprintf(&b, "%d %d\n", pf.LeafCount, len(pf.Portals))
	for _, p := range pf.Portals {
		fmt.Fprintf(&b, "%d %d %d\n", len(p.Points), p.Leafs[0], p.Leafs[1])
		for _, pt := range p.Points {
			fmt.Fprintf(&b, "( %g %g %g )\n", pt.X, pt.Y, pt.Z)
		}
	}
	return []byte(b.String())
}

// Write writes the serialized PRT1 text into w.
func (pf *PortalFile) Write(w io.Writer) error {
	data := pf.Serialize()
	_, err := w.Write(data)
	return err
}

type prtTokenReader struct {
	sc    *bufio.Scanner
	toks  []string
	index int
}

func newPrtTokenReader(r io.Reader) *prtTokenReader {
	tr := &prtTokenReader{sc: bufio.NewScanner(r)}
	tr.sc.Buffer(make([]byte, 4096), 1<<20)
	return tr
}

func (t *prtTokenReader) fill() bool {
	for t.index >= len(t.toks) {
		if !t.sc.Scan() {
			return false
		}
		t.toks = strings.Fields(t.sc.Text())
		t.index = 0
	}
	return true
}

func (t *prtTokenReader) Next() (string, bool) {
	if !t.fill() {
		return "", false
	}
	v := t.toks[t.index]
	t.index++
	return v, true
}

func (t *prtTokenReader) Peek() (string, bool) {
	if !t.fill() {
		return "", false
	}
	return t.toks[t.index], true
}

func (t *prtTokenReader) Int() (int64, error) {
	s, ok := t.Next()
	if !ok {
		return 0, io.EOF
	}
	return strconv.ParseInt(s, 10, 64)
}
