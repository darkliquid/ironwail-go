package vis

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Portal is one PRT1 portal: the shared facet polygon between two leaves.
type Portal struct {
	Leafs  [2]int
	Points [][3]float64
}

// portalFile mirrors qbsp.PortalFile for the vis side.
type portalFile struct {
	LeafCount int
	Portals   []Portal
}

// loadPrtFile parses a PRT1 portal file ("PRT1", <leafcount> <numportals>,
// then per portal "<numpoints> <leaf0> <leaf1>" and "( x y z )" points).
// Parenthesised points are required by ericw's reader but tolerated here.
func loadPrtFile(r io.Reader) (*portalFile, error) {
	tok := newTokenReader(r)
	magic, ok := tok.Next()
	if !ok {
		return nil, fmt.Errorf("vis: empty portal file")
	}
	if magic != "PRT1" && magic != "PRT1-AM" {
		return nil, fmt.Errorf("vis: unknown portal file header %q", magic)
	}
	pf := &portalFile{}
	lc, err := tok.Int()
	if err != nil {
		return nil, err
	}
	num, err := tok.Int()
	if err != nil {
		return nil, err
	}
	pf.LeafCount = int(lc)
	for i := 0; i < int(num); i++ {
		np, err := tok.Int()
		if err != nil {
			return nil, err
		}
		l0, err := tok.Int()
		if err != nil {
			return nil, err
		}
		l1, err := tok.Int()
		if err != nil {
			return nil, err
		}
		if int(l0) > pf.LeafCount || int(l1) > pf.LeafCount {
			return nil, fmt.Errorf("vis: portal %d leaf out of bounds", i)
		}
		p := Portal{Leafs: [2]int{int(l0), int(l1)}}
		for j := 0; j < int(np); j++ {
			// skip an optional "(" token
			nxt, _ := tok.Next()
			if nxt == "(" {
				nxt, _ = tok.Next()
			}
			x, err := strconv.ParseFloat(nxt, 64)
			if err != nil {
				return nil, err
			}
			y, _ := strconv.ParseFloat(tok.mustNext(), 64)
			zStr, _ := tok.Next()
			z, err := strconv.ParseFloat(zStr, 64)
			if err != nil {
				return nil, err
			}
			// consume optional ")"
			if nz, ok := tok.Peek(); ok && nz == ")" {
				_, _ = tok.Next()
			}
			p.Points = append(p.Points, [3]float64{x, y, z})
		}
		if len(p.Points) < 3 {
			return nil, fmt.Errorf("vis: portal %d degenerate winding", i)
		}
		pf.Portals = append(pf.Portals, p)
	}
	return pf, nil
}

// tokenReader is a whitespace tokeniser over the portal file.
type tokenReader struct {
	sc    *bufio.Scanner
	toks  []string
	index int
}

func newTokenReader(r io.Reader) *tokenReader {
	tr := &tokenReader{sc: bufio.NewScanner(r)}
	tr.sc.Buffer(make([]byte, 4096), 1<<20)
	return tr
}

func (t *tokenReader) fill() bool {
	for t.index >= len(t.toks) {
		if !t.sc.Scan() {
			return false
		}
		t.toks = strings.Fields(t.sc.Text())
		t.index = 0
	}
	return true
}

func (t *tokenReader) Next() (string, bool) {
	if !t.fill() {
		return "", false
	}
	v := t.toks[t.index]
	t.index++
	return v, true
}

func (t *tokenReader) mustNext() string {
	s, _ := t.Next()
	return s
}

func (t *tokenReader) Peek() (string, bool) {
	if !t.fill() {
		return "", false
	}
	return t.toks[t.index], true
}

// Int reads the next token as an integer.
func (t *tokenReader) Int() (int, error) {
	s, ok := t.Next()
	if !ok {
		return 0, io.ErrUnexpectedEOF
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("vis: bad integer %q", s)
	}
	return v, nil
}

var _ = bytes.Buffer{}