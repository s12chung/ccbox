// Package ptrx holds pointer utilities beyond the standard lib's bare pointers.
package ptrx

// Deref reads the pointee, the zero value on nil — a pointer field's unset view.
func Deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}
