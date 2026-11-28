// Package slicex holds slice utilities beyond the standard slices package.
package slicex

import (
	"slices"
	"strconv"
)

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

// Filter returns s's entries f reports true for, keeping order.
func Filter[E any](s []E, f func(E) bool) []E {
	var out []E
	for _, e := range s {
		if f(e) {
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

// Map renders each element with f, keeping order — nil for none.
func Map[E, R any](s []E, f func(E) R) []R {
	if len(s) == 0 {
		return nil
	}
	out := make([]R, len(s))
	for i, e := range s {
		out[i] = f(e)
	}
	return out
}

// ToMap renders each element as a key-value pair, later pairs overwriting earlier ones.
func ToMap[E any, K comparable, V any](s []E, pair func(E) (K, V)) map[K]V {
	m := make(map[K]V, len(s))
	for _, e := range s {
		k, v := pair(e)
		m[k] = v
	}
	return m
}

// SortNumbers returns s's digit strings in numeric order — "80" sorts after "3000".
func SortNumbers(s []string) []string {
	numbers := make([]int, len(s))
	for i, entry := range s {
		numbers[i], _ = strconv.Atoi(entry) // digit strings only
	}
	slices.Sort(numbers)
	return Map(numbers, strconv.Itoa)
}
