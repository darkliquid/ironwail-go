package mapfile

import (
	"bytes"
	"strings"
	"testing"
)

// equalCanonical compares the canonical brush definition: entity/epair order
// and per-face (Points, TexName, Vecs). TexDef surface form may differ after a
// rewrite (rotation/scale folded into axes), Vecs may not.
func equalCanonical(a, b *Map) bool {
	if len(a.Entities) != len(b.Entities) {
		return false
	}
	for i := range a.Entities {
		ea, eb := &a.Entities[i], &b.Entities[i]
		if len(ea.Epairs) != len(eb.Epairs) || len(ea.Brushes) != len(eb.Brushes) {
			return false
		}
		for j := range ea.Epairs {
			if ea.Epairs[j] != eb.Epairs[j] {
				return false
			}
		}
		for j := range ea.Brushes {
			ba, bb := &ea.Brushes[j], &eb.Brushes[j]
			if len(ba.Faces) != len(bb.Faces) {
				return false
			}
			for k := range ba.Faces {
				fa, fb := &ba.Faces[k], &bb.Faces[k]
				if fa.Points != fb.Points || fa.TexName != fb.TexName || fa.Vecs != fb.Vecs {
					return false
				}
			}
		}
	}
	return true
}

func TestWriteParseRoundTrip(t *testing.T) {
	m1, err := Parse(strings.NewReader(valve220BoxFixture))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, m1, WriteOptions{}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	m2, err := Parse(&buf)
	if err != nil {
		t.Fatalf("re-parse of written output: %v\n%s", err, buf.String())
	}
	if !equalCanonical(m1, m2) {
		t.Fatalf("round trip mismatch, wrote:\n%s", buf.String())
	}
}

func TestWriteGridSnap(t *testing.T) {
	src := strings.Replace(valve220BoxFixture, "( 0 64 0 ) ( 0 64 64 ) brick", "( 0 65 0 ) ( 0 65 66 ) brick", 1)
	m, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, m, WriteOptions{GridSnap: 8}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "65") || strings.Contains(out, "66") {
		t.Fatalf("unsnapped points in output:\n%s", out)
	}
	if !strings.Contains(out, "( 0 64 0 ) ( 0 64 64 )") {
		t.Fatalf("expected snapped points in output:\n%s", out)
	}
}

func TestWriteBrushAdaptiveGridSnap(t *testing.T) {
	// A 4-unit thin slab at z in [-24, -20] (like dm1's trim plates).
	// Under a blind 8-unit snap, z=-20 rounds to -24, collapsing the brush
	// thickness to 0, producing degenerate and duplicate planes.
	// Under adaptive snapping, it should adapt down to grid 4 and preserve all 6 faces.
	thinSlabSrc := `// entity 0
{
"classname" "worldspawn"
{
( 952 1448 -20 ) ( 952 1448 -24 ) ( 968 1448 -24 ) clip [ 0 0 0 0 ] [ 0 0 0 0 ] 0 1 1
( 968 1448 -20 ) ( 968 1448 -24 ) ( 968 1384 -24 ) METAL1_3 [ 0 1 0 0 ] [ 0 0 -1 0 ] 0 1 1
( 968 1448 -24 ) ( 952 1448 -24 ) ( 952 1384 -24 ) METAL1_3 [ 0 0 0 0 ] [ 0 0 0 0 ] 0 1 1
( 968 1384 -20 ) ( 968 1384 -24 ) ( 952 1384 -24 ) METAL1_3 [ 0 0 0 0 ] [ 0 0 0 0 ] 0 1 1
( 952 1384 -20 ) ( 952 1384 -24 ) ( 952 1448 -24 ) METAL1_3 [ 0 0 0 0 ] [ 0 0 0 0 ] 0 1 1
( 952 1448 -20 ) ( 968 1448 -20 ) ( 968 1384 -20 ) METAL1_3 [ 0 0 0 0 ] [ 0 0 0 0 ] 0 1 1
}
}
`
	m, err := Parse(strings.NewReader(thinSlabSrc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var buf bytes.Buffer
	if err := Write(&buf, m, WriteOptions{GridSnap: 8}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	reparsed, err := Parse(&buf)
	if err != nil {
		t.Fatalf("Parse written map: %v\nOutput was:\n%s", err, buf.String())
	}
	if len(reparsed.Entities) == 0 || len(reparsed.Entities[0].Brushes) == 0 {
		t.Fatalf("expected brush to survive, got: %+v", reparsed)
	}
	b := reparsed.Entities[0].Brushes[0]
	if len(b.Faces) != 6 {
		t.Fatalf("expected 6 faces after adaptive snap, got %d faces (collapsed under grid snap)", len(b.Faces))
	}
}