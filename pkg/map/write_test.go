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