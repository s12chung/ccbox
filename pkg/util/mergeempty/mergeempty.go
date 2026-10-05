// Package mergeempty combines two collections of the same type into one. The value helpers return nil
// only when both inputs are nil, so a non-nil empty input yields a non-nil empty result — callers that
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

// Merge overlays src's set fields over dst in place: lists append, maps merge per key,
// struct pointers set on both sides recurse per field, a set scalar or pointer wins — a field
// zero in src keeps dst's. Unexported fields are unwritable by reflection, so dst keeps its
// own. Every write deep-copies src, so the merge never aliases it.
func Merge[T any](dst *T, src T) {
	overlay(reflect.ValueOf(dst).Elem(), reflect.ValueOf(src))
}

// Struct overlays b onto a in a fresh struct, per-field like Merge — but unexported fields
// come back zero, never either side's: reflection can't write them, and the deep copy leaves
// them zero.
func Struct[T any](a, b *T) *T {
	switch {
	case a == nil && b == nil:
		return nil
	case a == nil:
		return deepcopy.Of(b)
	case b == nil:
		return deepcopy.Of(a)
	}
	out := *deepcopy.Of(a) // Of(a) is *T: the pointee is fresh, so the shallow deref copy shares nothing with a
	Merge(&out, *b)
	return &out
}

// overlay writes src's set fields over dst's, recursing into struct pointers set on both
// sides — lists append and maps merge per key, like Slice and Map
func overlay(dst, src reflect.Value) {
	for i := range src.NumField() {
		d, s := dst.Field(i), src.Field(i)
		if !d.CanSet() {
			continue // unexported: unwritable — dst keeps its own, per Merge's doc
		}
		if s.IsZero() {
			continue // a field unset in src inherits
		}
		if d.IsZero() {
			d.Set(deepcopy.Value(s)) // dst unset: src's, fresh
			continue
		}
		switch {
		case s.Kind() == reflect.Pointer && s.Elem().Kind() == reflect.Struct:
			overlay(d.Elem(), s.Elem()) // nested sections merge per field
			continue
		case s.Kind() == reflect.Slice:
			merged := reflect.MakeSlice(s.Type(), 0, d.Len()+s.Len())
			for i := range d.Len() {
				merged = reflect.Append(merged, deepcopy.Value(d.Index(i)))
			}
			for i := range s.Len() {
				merged = reflect.Append(merged, deepcopy.Value(s.Index(i)))
			}
			d.Set(merged)
			continue
		case s.Kind() == reflect.Map:
			merged := reflect.MakeMapWithSize(s.Type(), d.Len()+s.Len())
			for _, k := range d.MapKeys() {
				merged.SetMapIndex(k, deepcopy.Value(d.MapIndex(k)))
			}
			for _, k := range s.MapKeys() { // last write wins: src beats dst on collisions
				merged.SetMapIndex(k, deepcopy.Value(s.MapIndex(k)))
			}
			d.Set(merged)
			continue
		}
		d.Set(deepcopy.Value(s)) // fresh copy, so the merged layers never alias
	}
}
