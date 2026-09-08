package qbsp

import (
	"io"

	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// The .map parser and its types live in pkg/map (package mapfile) so the
// decompiler can share them; these aliases keep that single source of truth
// without changing this package's API.
type (
	Map      = mapfile.Map
	Entity   = mapfile.Entity
	Epair    = mapfile.Epair
	MapBrush = mapfile.MapBrush
	MapFace  = mapfile.MapFace
	TexDef   = mapfile.TexDef
)

// ParseMap parses a Quake .map file (QuakeEd and Valve 220 texture formats).
func ParseMap(r io.Reader) (*Map, error) { return mapfile.Parse(r) }