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

// Map renders each element with f, keeping order.
func Map[E, R any](s []E, f func(E) R) []R {
	out := make([]R, len(s))
	for i, e := range s {
		out[i] = f(e)
	}
	return out
}
