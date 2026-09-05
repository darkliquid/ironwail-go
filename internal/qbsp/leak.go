package qbsp

import (
	"strconv"

	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// detectLeak flood-fills the void ring outward through non-solid cells and
// reports whether any point entity's origin is reachable from outside the
// map (a leak), plus a trail of cell centres from the entity to the void.
func (c *compiler) detectLeak(m *Map, arr *arrangement) ([]vec3, bool) {
	// Ring cells: cells whose geometric facet on a box plane is non-empty
	// (they touch the outside of the map).
	n := len(arr.cells)
	parents := make([]int, n)
	visited := make([]bool, n)
	var queue []int

	for ci := range arr.cells {
		for _, h := range arr.cells[ci].hs {
			if h.plane >= len(c.planes)-6 { // one of the six box planes
				if c.facetNonEmpty(arr, ci, h.plane) {
					if !visited[ci] {
						visited[ci] = true
						queue = append(queue, ci)
					}
					break
				}
			}
		}
	}

	for qi := 0; qi < len(queue); qi++ {
		ci := queue[qi]
		cell := arr.cells[ci]
		if cell.content == bsp.ContentsSolid {
			continue
		}
		for _, h := range cell.hs {
			ni := arr.neighborPlane(ci, h.plane)
			if ni < 0 || visited[ni] {
				continue
			}
			if arr.cells[ni].content == bsp.ContentsSolid {
				continue
			}
			visited[ni] = true
			parents[ni] = ci
			queue = append(queue, ni)
		}
	}

	// Every point entity with an origin: reachable from the void = leak.
	for _, ent := range m.Entities[1:] {
		originStr, ok := ent.Value("origin")
		if !ok {
			continue
		}
		origin, err := parseOrigin(originStr)
		if err != nil {
			continue
		}
		ci := arr.cellAt(origin)
		if ci < 0 {
			// entity outside the map bounds entirely
			return []vec3{origin}, true
		}
		if visited[ci] {
			// walk the parent chain back to the ring for a point trail
			var trail []vec3
			cur := ci
			for cur >= 0 && !c.isRingCell(arr, cur) {
				trail = append([]vec3{arr.cellCenter(cur)}, trail...)
				cur = parents[cur]
			}
			trail = append(trail, arr.cellCenter(cur))
			return trail, true
		}
	}
	return nil, false
}

// isRingCell reports whether the cell has a non-empty facet on one of the
// six box planes (i.e. it touches the void).
func (c *compiler) isRingCell(arr *arrangement, ci int) bool {
	for _, h := range arr.cells[ci].hs {
		if h.plane >= len(c.planes)-6 {
			if c.facetNonEmpty(arr, ci, h.plane) {
				return true
			}
		}
	}
	return false
}

// facetNonEmpty reports whether the cell's facet on plane pi is a real
// polygon (>= 3 points).
func (c *compiler) facetNonEmpty(arr *arrangement, ci, pi int) bool {
	for _, w := range arr.cells[ci].facets(arr.planes, arr.bounds) {
		pl := c.planes[pi]
		matches := 0
		for _, p := range w {
			if planeSide(pl, p) == 0 {
				matches++
			}
		}
		if matches == len(w) && len(w) >= 3 {
			return true
		}
	}
	return false
}

// parseOrigin parses a "x y z" entity origin.
func parseOrigin(s string) (vec3, error) {
	var o vec3
	n, err := scanVec3(s, &o)
	if err != nil || n != 3 {
		return vec3{}, err
	}
	return o, nil
}

// scanVec3 scans up to three whitespace-separated floats into v.
func scanVec3(s string, v *vec3) (int, error) {
	var vals [3]float64
	var err error
	n := 0
	idx := 0
	for idx < len(s) && n < 3 {
		for idx < len(s) && (s[idx] == ' ' || s[idx] == '\t' || s[idx] == '\n') {
			idx++
		}
		if idx >= len(s) {
			break
		}
		start := idx
		for idx < len(s) && s[idx] != ' ' && s[idx] != '\t' && s[idx] != '\n' {
			idx++
		}
		vals[n], err = strconv.ParseFloat(s[start:idx], 64)
		if err != nil {
			return n, err
		}
		n++
	}
	if n >= 1 {
		v[0] = vals[0]
	}
	if n >= 2 {
		v[1] = vals[1]
	}
	if n >= 3 {
		v[2] = vals[2]
	}
	return n, nil
}