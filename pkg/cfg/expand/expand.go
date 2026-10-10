// Package expand holds the config's entry expansions: a keyed entry expands to its
// expansion, the rest keep as-is
package expand

import "github.com/s12chung/ccbox/pkg/util/mapx"

// Expansion is a keyed entry's expansion: Key renames the entry's key, Value resolves
// its raw value
type Expansion struct {
	Key   string
	Value func(value string) string
}

// Map expands each entry found in expansions to its Expansion's resolved key and value,
// the rest ride expand
func Map(entries map[string]string, expansions map[string]Expansion, expand func(key, value string) (string, string)) map[string]string {
	expanded := make(map[string]string, len(entries))
	for key, value := range entries {
		if e, ok := expansions[key]; ok {
			key, value = e.Key, e.Value(value)
		} else {
			key, value = expand(key, value)
		}
		expanded[key] = value
	}
	return mapx.NilIfEmpty(expanded)
}

// Slice expands each entry found in expansions to its expansion, keeping the other
// entries as-is — order kept, repeats dropped
func Slice(entries []string, expansions map[string][]string) []string {
	var out []string
	seen := map[string]bool{}
	for _, entry := range entries {
		expanded, ok := expansions[entry]
		if !ok {
			expanded = []string{entry}
		}
		for _, e := range expanded {
			if seen[e] {
				continue
			}
			seen[e] = true
			out = append(out, e)
		}
	}
	return out
}
