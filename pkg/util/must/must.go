// Package must contains helpers that panic on error, for unreachable
// or setup-time-fatal errors.
package must

// Get returns v, panicking on err.
func Get[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// Do panics on err.
func Do(err error) {
	if err != nil {
		panic(err)
	}
}
