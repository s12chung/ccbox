// Package mergeempty combines two collections of the same type into one. Each helper returns nil only
// when both inputs are nil, so a non-nil empty input yields a non-nil empty result — callers that
// treat nil and empty differently keep that distinction, unlike the standard slices.Concat and
// maps.Copy patterns that lose it.
package mergeempty

import (
	"maps"
	"reflect"

	"github.com/s12chung/ccbox/pkg/util/deepcopy"
)

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

// Ptr overlays b onto a in a fresh pointer — b wins when set, nil b keeps a — deep-copying
// the winner, so overlaid pointer fields never share their pointee across layers.
func Ptr[T any](a, b *T) *T {
	winner := b
	if winner == nil {
		winner = a
	}
	if winner == nil {
		return nil
	}
	return deepcopy.Of(winner)
}

// Struct overlays b onto a in a fresh struct — b winning on set fields, a field unset in b
// keeping a's — recursing into struct pointers set on both sides, so nested sections merge
// per field like Map merges per key. Unexported fields come back zero, never either
// side's — reflection can't write them, and the deep copy leaves them zero.
func Struct[T any](a, b *T) *T {
	switch {
	case a == nil && b == nil:
		return nil
	case a == nil:
		return deepcopy.Of(b)
	case b == nil:
		return deepcopy.Of(a)
	}
	out := deepcopy.Of(a)
	overlay(reflect.ValueOf(out).Elem(), reflect.ValueOf(b).Elem())
	return out
}

// overlay writes src's set fields over dst's, recursing into struct pointers set on both
// sides — dst is a fresh copy, so the writes never reach the inputs
func overlay(dst, src reflect.Value) {
	for i := range src.NumField() {
		d, s := dst.Field(i), src.Field(i)
		if !d.CanSet() {
			continue // unexported: unwritable — and already zero, per Struct's doc
		}
		if s.IsZero() {
			continue // a field unset in src inherits
		}
		if s.Kind() == reflect.Pointer && !d.IsNil() && s.Elem().Kind() == reflect.Struct {
			overlay(d.Elem(), s.Elem()) // nested sections merge per field
			continue
		}
		d.Set(deepcopy.Value(s)) // fresh copy, so the merged layers never alias
	}
}
