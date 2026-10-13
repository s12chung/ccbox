// Package slicex holds slice utilities beyond the standard slices package.
package slicex

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

// Intersect returns a's entries also in b, keeping order.
func Intersect[E comparable](a, b []E) []E {
	var out []E
	for _, e := range a {
		if slices.Contains(b, e) {
			out = append(out, e)
		}
	}
	return out
}

// Unique returns s's entries in order, repeats dropped.
func Unique[E comparable](s []E) []E {
	var out []E
	for _, e := range s {
		if !slices.Contains(out, e) {
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
