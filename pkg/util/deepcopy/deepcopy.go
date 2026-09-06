// Package deepcopy duplicates values so the copy shares no slice, map, or pointer backing storage
// with the original.
package deepcopy

import "reflect"

// Of returns a deep copy of v. Slices, maps, pointers, and structs are duplicated recursively;
// everything else is copied by value. Nil slices/maps/pointers stay nil. Unexported struct
// fields are left zero — reflection can't set them.
func Of[T any](v T) T {
	// copyValue preserves v's type, so Set doubles as the type check — no assertion needed
	var out T
	reflect.ValueOf(&out).Elem().Set(copyValue(reflect.ValueOf(v)))
	return out
}

func copyValue(v reflect.Value) reflect.Value {
	switch v.Kind() {
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := range v.Len() {
			out.Index(i).Set(copyValue(v.Index(i)))
		}
		return out
	case reflect.Map:
		if v.IsNil() {
			return v
		}
		out := reflect.MakeMap(v.Type())
		for _, k := range v.MapKeys() {
			out.SetMapIndex(k, copyValue(v.MapIndex(k)))
		}
		return out
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		out := reflect.New(v.Type().Elem())
		out.Elem().Set(copyValue(v.Elem()))
		return out
	case reflect.Struct:
		out := reflect.New(v.Type()).Elem()
		for i := range v.NumField() {
			if f := out.Field(i); f.CanSet() {
				f.Set(copyValue(v.Field(i)))
			}
		}
		return out
	default:
		return v
	}
}
