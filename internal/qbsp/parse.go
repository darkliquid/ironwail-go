package qbsp

import "strconv"

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