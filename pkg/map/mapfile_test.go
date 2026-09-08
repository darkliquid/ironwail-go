package mapfile

import (
	"strings"
	"testing"
)

const valve220BoxFixture = `// entity 0
{
"classname" "worldspawn"
{
( 0 0 0 ) ( 0 64 0 ) ( 0 64 64 ) brick [ 0 1 0 0 ] [ 0 0 -1 0 ] 0 1 1
( 64 0 0 ) ( 64 0 64 ) ( 64 64 64 ) brick [ 0 1 0 0 ] [ 0 0 -1 0 ] 0 1 1
( 0 0 0 ) ( 64 0 0 ) ( 64 0 64 ) brick [ 1 0 0 0 ] [ 0 0 -1 0 ] 0 1 1
( 0 64 0 ) ( 0 64 64 ) ( 64 64 64 ) brick [ 1 0 0 0 ] [ 0 0 -1 0 ] 0 1 1
( 0 0 0 ) ( 0 64 0 ) ( 64 64 0 ) brick [ 1 0 0 0 ] [ 0 -1 0 0 ] 0 1 1
( 0 0 64 ) ( 64 0 64 ) ( 64 64 64 ) brick [ 1 0 0 0 ] [ 0 -1 0 0 ] 0 1 1
}
}
`

func TestParseValve220Box(t *testing.T) {
	m, err := Parse(strings.NewReader(valve220BoxFixture))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Entities) != 1 {
		t.Fatalf("entities = %d, want 1", len(m.Entities))
	}
	e := m.Entities[0]
	if v, ok := e.Value("classname"); !ok || v != "worldspawn" {
		t.Fatalf("classname = %q, %v", v, ok)
	}
	if len(e.Brushes) != 1 {
		t.Fatalf("brushes = %d, want 1", len(e.Brushes))
	}
	if got := len(e.Brushes[0].Faces); got != 6 {
		t.Fatalf("faces = %d, want 6", got)
	}
	f := e.Brushes[0].Faces[0]
	if f.TexName != "brick" {
		t.Fatalf("texname = %q, want brick", f.TexName)
	}
	if f.Tex.QuakeEd {
		t.Fatal("expected Valve 220 texdef")
	}
	// ericw convention: normal = normalize(cross(p0-p1, p2-p1)) = (-1,0,0)
	if f.Normal != (Vec3{X: -1, Y: 0, Z: 0}) {
		t.Fatalf("normal = %v, want [-1 0 0]", f.Normal)
	}
	if f.Dist != 0 {
		t.Fatalf("dist = %v, want 0", f.Dist)
	}
}