// Package uslice holds slice utilities beyond the standard slices package.
package uslice

import "slices"

// Minus returns a's entries absent from b, keeping order.
func Minus[E comparable](a, b []E) []E {
	var out []E
	for _, e := range a {
		if !slices.Contains(b, e) {
			out = append(out, e)
		}
	}
	return out
}
