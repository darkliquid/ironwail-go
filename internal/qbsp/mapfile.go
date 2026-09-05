package qbsp

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// Map is a parsed .map file: an ordered list of entities (worldspawn first
// in Quake convention).
type Map struct {
	Entities []Entity
}

// Entity is one "{}" block: key/value pairs followed by brushes. Epairs
// preserves file order; lookups return the last value for a key, matching
// ericw's entdict.
type Entity struct {
	Epairs  []Epair
	Brushes []MapBrush
}

// Epair is one key "value" line in an entity dictionary. Keys are
// whitespace-trimmed at parse time (matching ericw).
type Epair struct {
	Key   string
	Value string
}

// Value returns the last value stored under key.
func (e *Entity) Value(key string) (string, bool) {
	for i := len(e.Epairs) - 1; i >= 0; i-- {
		if e.Epairs[i].Key == key {
			return e.Epairs[i].Value, true
		}
	}
	return "", false
}

// MapBrush is a "{ face face ... }" brush inside an entity.
type MapBrush struct {
	Faces []MapFace
	// Line is the source line of the brush's first face, for diagnostics.
	Line int
}

// MapFace is one brush side: three plane points, the derived plane, the
// texture name, and the texture definition with its computed s/t vectors.
type MapFace struct {
	// Points are the raw .map plane points (float64 precision).
	Points [3]vec3
	// Normal and Dist are the derived plane (ericw convention:
	// normalize(cross(p0-p1, p2-p1)), dist = dot(p1, normal)).
	Normal vec3
	Dist   float64
	// TexName is the raw texture name from the map.
	TexName string
	// Tex is the parsed texture definition (QuakeEd or Valve 220).
	Tex TexDef
	// Vecs are the computed texture axes: Vecs[0] = s vector + soffset
	// ([4] = sxyz, soffset), Vecs[1] = t vector + toffset.
	Vecs [2][4]float64
	// Line is the source line of the face, for warnings.
	Line int
}

// Plane returns the face's derived plane.
func (f *MapFace) Plane() plane { return plane{Normal: f.Normal, Dist: f.Dist} }

// TexDef is a texture definition as written in the map. Valve 220 faces
// carry an explicit axis; QuakeEd faces derive theirs from the plane via
// the QuakeEd baseaxis table at vecs-computation time.
type TexDef struct {
	// QuakeEd is true for the six-token shiftX shiftY rot scaleX scaleY
	// form, false for the Valve 220 axis form.
	QuakeEd bool
	ShiftX  float64
	ShiftY  float64
	Rotate  float64
	ScaleX  float64
	ScaleY  float64
	// Axis are the Valve 220 u/v axes (only when !QuakeEd).
	Axis [2]vec3
}

// ---- tokenizer ----

type parseFlags int

const (
	parseNone     parseFlags = 0
	parseSameLine parseFlags = 1 << iota
	parseOptional
)

// mapParser is a byte-level tokenizer matching ericw's parser_t: tokens are
// whitespace-delimited, quoted strings span double quotes, comment lines
// ("//" and ";") are skipped, and line bounds can be requested via flags.
type mapParser struct {
	src  []byte
	pos  int
	line int
}

func newMapParser(data []byte) *mapParser {
	return &mapParser{src: data, line: 1}
}

// skipSpace consumes whitespace/comments. It handles the EOF and EOL
// behaviour for the given flags: optional returns no token at EOL/EOF,
// sameLine errors past a newline.
func (p *mapParser) skipSpace(flags parseFlags) error {
	sameLine := flags&parseSameLine != 0
	optional := flags&parseOptional != 0
	for {
		if p.pos >= len(p.src) {
			if optional {
				return nil
			}
			return io.EOF
		}
		c := p.src[p.pos]
		if c <= 32 {
			if c == '\n' {
				if optional {
					return nil
				}
				if sameLine {
					return fmt.Errorf("line %d: line is incomplete", p.line)
				}
				p.line++
			}
			p.pos++
			continue
		}
		if c == '/' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '/' {
			// line comment
			if optional {
				return nil
			}
			if sameLine {
				return fmt.Errorf("line %d: line is incomplete", p.line)
			}
			for p.pos < len(p.src) && p.src[p.pos] != '\n' {
				p.pos++
			}
			continue
		}
		if c == ';' {
			// QuArK writes ; comments in Q2 maps
			if optional {
				return nil
			}
			if sameLine {
				return fmt.Errorf("line %d: line is incomplete", p.line)
			}
			for p.pos < len(p.src) && p.src[p.pos] != '\n' {
				p.pos++
			}
			continue
		}
		return nil
	}
}

// next reads one token. Returns io.EOF when the input is exhausted and the
// token was not optional.
func (p *mapParser) next(flags parseFlags) (string, error) {
	if err := p.skipSpace(flags); err != nil {
		return "", err
	}
	if p.pos >= len(p.src) {
		if flags&parseOptional != 0 {
			return "", nil
		}
		return "", io.EOF
	}

	tok, err := p.copyToken()
	if err != nil {
		return "", err
	}
	return tok, nil
}

// peekNext reads the next token without consuming it (parser state is
// restored). EOL/EOF yields ("", false) — a lenient superset of ericw's
// peek, used for brush-format detection (any line) and the Valve-bracket
// lookahead (same line, via flags).
func (p *mapParser) peekNext(flags parseFlags) (string, bool) {
	savePos, saveLine := p.pos, p.line
	tok, err := p.next(flags)
	p.pos, p.line = savePos, saveLine
	if err != nil || tok == "" {
		return "", false
	}
	return tok, true
}

// copyToken reads a quoted or bare token at the current position (which
// must point at a non-space byte).
func (p *mapParser) copyToken() (string, error) {
	var b strings.Builder
	if p.src[p.pos] == '"' {
		p.pos++
		for {
			if p.pos >= len(p.src) {
				return "", fmt.Errorf("line %d: EOF inside quoted token", p.line)
			}
			c := p.src[p.pos]
			if c == '"' {
				p.pos++
				return b.String(), nil
			}
			if c == '\\' && p.pos+1 < len(p.src) {
				next := p.src[p.pos+1]
				switch {
				case next == 'x' || (next >= '0' && next <= '9'):
					// drop the backslash, keep the character
					p.pos++
				case next == 'n' || next == '\'' || next == 'r' || next == 't' ||
					next == '\\' || next == 'b' || next == '"':
					// keep backslash + escaped character
					b.WriteByte(c)
				default:
					// unknown escape: drop the backslash
					p.pos++
				}
			}
			if c == '\n' {
				p.line++
			}
			b.WriteByte(c)
			p.pos++
		}
	}
	for p.pos < len(p.src) && p.src[p.pos] > 32 {
		b.WriteByte(p.src[p.pos])
		p.pos++
	}
	return b.String(), nil
}

func parseNumber(p *mapParser, flags parseFlags, what string) (float64, error) {
	tok, err := p.next(flags)
	if err != nil {
		return 0, fmt.Errorf("line %d: %s: %v", p.line, what, err)
	}
	if tok == "" {
		return 0, fmt.Errorf("line %d: %s: missing value", p.line, what)
	}
	v, err := strconv.ParseFloat(tok, 64)
	if err != nil {
		return 0, fmt.Errorf("line %d: %s: bad number %q", p.line, what, tok)
	}
	return v, nil
}

// ---- parsing ----

// ParseMap parses a Quake .map file (QuakeEd base format; individual faces
// may use Valve 220 axis syntax). Quake 3 brush-primitives maps are
// rejected with a clear error. The world entity, point entities, and brush
// entities are returned in file order, mirroring ericw's mapfile parser.
func ParseMap(r io.Reader) (*Map, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	p := newMapParser(data)
	m := &Map{}

	for {
		tok, err := p.next(parseNone)
		if err == io.EOF {
			return m, nil
		}
		if err != nil {
			return nil, err
		}
		if tok != "{" {
			return nil, fmt.Errorf("line %d: invalid entity format, \"{\" not found", p.line)
		}
		ent, err := p.parseEntity()
		if err != nil {
			return nil, err
		}
		m.Entities = append(m.Entities, ent)
	}
}

func (p *mapParser) parseEntity() (Entity, error) {
	var ent Entity
	for {
		tok, err := p.next(parseNone)
		if err == io.EOF {
			return ent, fmt.Errorf("line %d: unexpected EOF (no closing brace)", p.line)
		}
		if err != nil {
			return ent, err
		}
		switch tok {
		case "}":
			return ent, nil
		case "{":
			brush, err := p.parseBrush()
			if err != nil {
				return ent, err
			}
			ent.Brushes = append(ent.Brushes, brush)
		default:
			key := strings.TrimSpace(tok)
			val, err := p.next(parseSameLine)
			if err != nil {
				return ent, fmt.Errorf("line %d: key %q value: %v", p.line, key, err)
			}
			ent.Epairs = append(ent.Epairs, Epair{Key: key, Value: val})
		}
	}
}

func (p *mapParser) parseBrush() (MapBrush, error) {
	line := p.line
	// Detect brushes written in Quake 3 brush-primitives format: any token
	// other than "(" or "}" means it is not a QuakeEd brush.
	if tok, ok := p.peekNext(parseNone); ok && tok != "(" && tok != "}" {
		return MapBrush{}, fmt.Errorf("line %d: brush primitives format not supported (QuakeEd/Valve 220 only)", line)
	}

	var b MapBrush
	for {
		tok, err := p.next(parseNone)
		if err == io.EOF {
			return b, fmt.Errorf("line %d: unexpected EOF (no closing brace)", p.line)
		}
		if err != nil {
			return b, err
		}
		if tok == "}" {
			return b, nil
		}
		if tok != "(" {
			return b, fmt.Errorf("line %d: invalid brush plane format, got %q", p.line, tok)
		}
		face, skip, err := p.parseBrushFace(tok)
		if err != nil {
			return b, err
		}
		if skip {
			continue
		}
		if b.hasDuplicatePlane(face) {
			fmt.Printf("WARNING: line %d: Brush with duplicate plane\n", face.Line)
			continue
		}
		b.Faces = append(b.Faces, face)
		if len(b.Faces) == 1 {
			b.Line = face.Line
		}
	}
}

// hasDuplicatePlane reports whether face duplicates any existing brush face
// plane (in either orientation), matching ericw's epsilonEqual check.
func (b *MapBrush) hasDuplicatePlane(face MapFace) bool {
	flipped := plane{Normal: v3(-face.Normal[0], -face.Normal[1], -face.Normal[2]), Dist: -face.Dist}
	for _, existing := range b.Faces {
		if planeEqual(face.Plane(), existing.Plane()) || planeEqual(flipped, existing.Plane()) {
			return true
		}
	}
	return false
}

// parseBrushFace parses one brush side. tok must already be the first "(".
// skip reports a degenerate (zero-normal) or duplicate-plane face that the
// caller should drop, mirroring the warnings in ericw mapfile.cc.
func (p *mapParser) parseBrushFace(first string) (MapFace, bool, error) {
	line := p.line
	var f MapFace
	f.Line = line

	for i := 0; i < 3; i++ {
		if i == 0 {
			if first != "(" {
				return f, false, fmt.Errorf("line %d: invalid brush plane format", line)
			}
		} else {
			tok, err := p.next(parseNone)
			if err != nil || tok != "(" {
				return f, false, fmt.Errorf("line %d: invalid brush plane format", line)
			}
		}
		for j := 0; j < 3; j++ {
			v, err := parseNumber(p, parseSameLine, "plane point")
			if err != nil {
				return f, false, err
			}
			f.Points[i][j] = v
		}
		tok, err := p.next(parseSameLine)
		if err != nil || tok != ")" {
			return f, false, fmt.Errorf("line %d: invalid brush plane format", line)
		}
	}

	pl, length := planeFromPoints(f.Points[0], f.Points[1], f.Points[2])
	f.Normal, f.Dist = pl.Normal, pl.Dist

	// Texture definition comes before the degenerate check, exactly like
	// ericw: the tokens must be consumed so later faces parse cleanly even
	// when this side is dropped.
	if err := p.parseTextureDef(&f); err != nil {
		return f, false, err
	}

	if length < 0.000001 {
		// NORMAL_EPSILON: plane with no normal; warn and drop the side.
		fmt.Printf("WARNING: line %d: Brush plane with no normal\n", line)
		return f, true, nil
	}
	return f, false, nil
}

// parseTextureDef reads the texture name and either a Valve 220 axis
// definition or a QuakeEd shift/rot/scale definition (auto-detected per
// face by the leading "["), matching ericw's parse_texture_def.
func (p *mapParser) parseTextureDef(f *MapFace) error {
	tok, err := p.next(parseNone)
	if err != nil {
		return fmt.Errorf("line %d: texture name: %v", p.line, err)
	}
	f.TexName = tok

	if bracket, ok := p.peekNext(parseSameLine); ok && bracket == "[" {
		tex, err := p.parseValve220()
		if err != nil {
			return err
		}
		f.Tex = tex
	} else {
		tex, err := p.parseQuakeEd()
		if err != nil {
			return err
		}
		f.Tex = tex
	}

	// Optional Quake 2 extended surface info (contents flags value) and
	// QuArK //TX comments. Comments are skipped by the tokenizer; the
	// numbers must be consumed so the next face's first token is not
	// swallowed.
	for i := 0; i < 3; i++ {
		n, err := p.next(parseOptional)
		if err != nil || n == "" {
			break
		}
		if _, err := strconv.Atoi(n); err != nil {
			break
		}
	}

	f.computeVecs()
	return nil
}

func (p *mapParser) parseValve220() (TexDef, error) {
	var tex TexDef
	for i := 0; i < 2; i++ {
		tok, err := p.next(parseSameLine)
		if err != nil || tok != "[" {
			return tex, fmt.Errorf("line %d: couldn't parse Valve220 texture info", p.line)
		}
		for j := 0; j < 3; j++ {
			v, err := parseNumber(p, parseSameLine, "valve axis")
			if err != nil {
				return tex, err
			}
			tex.Axis[i][j] = v
		}
		shift, err := parseNumber(p, parseSameLine, "valve offset")
		if err != nil {
			return tex, err
		}
		if i == 0 {
			tex.ShiftX = shift
		} else {
			tex.ShiftY = shift
		}
		tok, err = p.next(parseSameLine)
		if err != nil || tok != "]" {
			return tex, fmt.Errorf("line %d: couldn't parse Valve220 texture info", p.line)
		}
	}
	var err error
	if tex.Rotate, err = parseNumber(p, parseSameLine, "valve rotate"); err != nil {
		return tex, err
	}
	if tex.ScaleX, err = parseNumber(p, parseSameLine, "valve scale"); err != nil {
		return tex, err
	}
	if tex.ScaleY, err = parseNumber(p, parseSameLine, "valve scale"); err != nil {
		return tex, err
	}
	return tex, nil
}

func (p *mapParser) parseQuakeEd() (TexDef, error) {
	var tex TexDef
	tex.QuakeEd = true
	var err error
	if tex.ShiftX, err = parseNumber(p, parseSameLine, "texdef shift"); err != nil {
		return tex, err
	}
	if tex.ShiftY, err = parseNumber(p, parseSameLine, "texdef shift"); err != nil {
		return tex, err
	}
	if tex.Rotate, err = parseNumber(p, parseSameLine, "texdef rotate"); err != nil {
		return tex, err
	}
	if tex.ScaleX, err = parseNumber(p, parseSameLine, "texdef scale"); err != nil {
		return tex, err
	}
	if tex.ScaleY, err = parseNumber(p, parseSameLine, "texdef scale"); err != nil {
		return tex, err
	}
	return tex, nil
}

// computeVecs derives the s/t texture vectors from the texdef and the face
// plane, reproducing ericw's set_texinfo math (baseaxis lookup for QuakeEd,
// axis/scale for Valve 220), including the near-integer rounding workaround.
func (f *MapFace) computeVecs() {
	tex := f.Tex
	sf := func(v float64) float64 {
		if v == 0 {
			return 1
		}
		return v
	}
	if !tex.QuakeEd {
		// Valve 220: axes are explicit.
		for i := 0; i < 3; i++ {
			f.Vecs[0][i] = tex.Axis[0][i] / sf(tex.ScaleX)
			f.Vecs[1][i] = tex.Axis[1][i] / sf(tex.ScaleY)
		}
		f.Vecs[0][3] = tex.ShiftX
		f.Vecs[1][3] = tex.ShiftY
	} else {
		xv, yv := quakeEdAxis(f.Normal)
		vectors := [2]vec3{xv, yv}

		ang := tex.Rotate / 180.0 * math.Pi
		sinv, cosv := math.Sin(ang), math.Cos(ang)

		sv := 0
		if vectors[0][0] == 0 && vectors[0][1] == 0 {
			sv = 2
		} else if vectors[0][0] == 0 {
			sv = 1
		}
		// tv: non-zero component of vectors[1]
		tv := 2
		for i := 0; i < 3; i++ {
			if vectors[1][i] != 0 {
				tv = i
				break
			}
		}

		for i := 0; i < 2; i++ {
			ns := cosv*vectors[i][sv] - sinv*vectors[i][tv]
			nt := sinv*vectors[i][sv] + cosv*vectors[i][tv]
			vectors[i][sv] = ns
			vectors[i][tv] = nt
		}

		scale := [2]float64{sf(tex.ScaleX), sf(tex.ScaleY)}
		for i := 0; i < 2; i++ {
			for j := 0; j < 3; j++ {
				f.Vecs[i][j] = vectors[i][j] / scale[i]
			}
		}
		f.Vecs[0][3] = tex.ShiftX
		f.Vecs[1][3] = tex.ShiftY
	}

	// Round values that are within ZERO_EPSILON of integers (the
	// DarkPlaces lightmap-size workaround in ericw).
	for i := 0; i < 2; i++ {
		for j := 0; j < 4; j++ {
			f.Vecs[i][j] = planeRoundNearInt(f.Vecs[i][j])
		}
	}
}

// quakeEdAxis returns the s/t axis pair for a face normal using the exact
// QuakeEd baseaxis[18] table (floor/ceiling/west/east/south/north). The
// best axis is the first maximising dot(normal, mainAxis) — old-axis
// behaviour, matching ericw's texture_axis_t with use_new_axis=false.
func quakeEdAxis(normal vec3) (xv, yv vec3) {
	baseaxis := [6][3]vec3{
		// floor
		{v3(0, 0, 1), v3(1, 0, 0), v3(0, -1, 0)},
		// ceiling
		{v3(0, 0, -1), v3(1, 0, 0), v3(0, -1, 0)},
		// west wall
		{v3(1, 0, 0), v3(0, 1, 0), v3(0, 0, -1)},
		// east wall
		{v3(-1, 0, 0), v3(0, 1, 0), v3(0, 0, -1)},
		// south wall
		{v3(0, 1, 0), v3(1, 0, 0), v3(0, 0, -1)},
		// north wall
		{v3(0, -1, 0), v3(1, 0, 0), v3(0, 0, -1)},
	}

	best := -1.0
	bestAxis := 0
	for i := 0; i < 6; i++ {
		dot := v3Dot(normal, baseaxis[i][0])
		if dot > best {
			best = dot
			bestAxis = i
		}
	}
	return baseaxis[bestAxis][1], baseaxis[bestAxis][2]
}