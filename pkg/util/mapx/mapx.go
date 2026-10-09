// Package mapx holds map utilities beyond the standard maps package.
package mapx

// Get returns m's value for k, ok false for a missing key. Simplified simple returns.
func Get[K comparable, V any](m map[K]V, k K) (V, bool) {
	v, ok := m[k]
	return v, ok
}

// NilIfEmpty collapses an empty map to nil, so unset and empty stay indistinguishable.
func NilIfEmpty[M ~map[K]V, K, V comparable](m M) M {
	if len(m) == 0 {
		return nil
	}
	return m
}
