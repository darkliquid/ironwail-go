package qbsp

import (
	"math"
	"testing"
)

func TestDivideBoundsAxial(t *testing.T) {
	b := [2]vec3{v3(0, 0, 0), v3(1000, 100, 1000)}
	f, bk := divideBounds(b, plane{Normal: v3(1, 0, 0), Dist: 500})
	if math.Abs(boundsVolume(f)-5e7) > 1 || math.Abs(boundsVolume(bk)-5e7) > 1 {
		t.Fatalf("axial split volumes wrong: front=%v back=%v", boundsVolume(f), boundsVolume(bk))
	}
	if math.Abs(splitPlaneMetric(plane{Normal: v3(1, 0, 0), Dist: 500}, b)) > 1 {
		t.Fatalf("balanced axial split metric != 0")
	}
}

func TestDivideBoundsNonAxial(t *testing.T) {
	b := [2]vec3{v3(0, 0, 0), v3(1000, 100, 1000)}
	// 45-degree X/Z cut through the box center: balanced volumes.
	n := v3(math.Sqrt2/2, 0, math.Sqrt2/2)
	p := plane{Normal: n, Dist: n.Dot(v3(500, 50, 500))}
	m := splitPlaneMetric(p, b)
	if m > 1e6*0.01+1 {
		t.Fatalf("centered 45-degree cut should be near-balanced, metric=%v", m)
	}
	// A sliver cut grazing one corner must score badly (near full volume).
	p2 := plane{Normal: n, Dist: n.Dot(v3(990, 50, 990))}
	m2 := splitPlaneMetric(p2, b)
	if m2 < boundsVolume(b)*0.25 {
		t.Fatalf("sliver cut should score near full volume, metric=%v full=%v", m2, boundsVolume(b))
	}
}
