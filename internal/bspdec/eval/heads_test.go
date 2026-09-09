package eval

import (
	"math"
	"testing"
)

func TestHeadroomGain(t *testing.T) {
	rows := []HeadroomRow{
		{PkgID: "a", BaselineIoU: 0.90, CeilingIoU: 0.99},
		{PkgID: "b", BaselineIoU: 0.92, CeilingIoU: 0.93},
	}
	_, mean := ComposeHeadroomReport(rows, true)
	if math.Abs(mean-0.05) > 1e-9 {
		t.Fatalf("mean gain = %v, want 0.05", mean)
	}
	if v := GoVerdict(mean, 0.02); v != "go" {
		t.Fatalf("verdict = %q, want go", v)
	}
	if v := GoVerdict(0.01, 0.02); v != "no-go" {
		t.Fatalf("verdict = %q, want no-go", v)
	}
}