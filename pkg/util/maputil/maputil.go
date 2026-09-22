// Package maputil holds map utilities beyond the standard maps package.
package maputil

// Get returns m's value for k, ok false for a missing key. Simplified simple returns.
func Get[K comparable, V any](m map[K]V, k K) (V, bool) {
	v, ok := m[k]
	return v, ok
}
