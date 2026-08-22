// Package mergeempty combines two collections of the same type into one. Each helper returns nil only
// when both inputs are nil, so a non-nil empty input yields a non-nil empty result — callers that
// treat nil and empty differently keep that distinction, unlike the standard slices.Concat and
// maps.Copy patterns that lose it.
package mergeempty

import "maps"

// Slice returns a's elements followed by b's in a fresh slice.
func Slice[T any](a, b []T) []T {
	if a == nil && b == nil {
		return nil
	}
	return append(append([]T{}, a...), b...)
}

// Map overlays b onto a in a fresh map, b winning on key collisions.
func Map[K comparable, V any](a, b map[K]V) map[K]V {
	if a == nil && b == nil {
		return nil
	}
	out := make(map[K]V, len(a)+len(b))
	maps.Copy(out, a)
	maps.Copy(out, b)
	return out
}
