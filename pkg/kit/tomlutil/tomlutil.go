// Package tomlutil contains TOML helpers
package tomlutil

import "strconv"

// Key quotes the key when the name isn't a TOML bare key (npm:playwright)
func Key(name string) string {
	bare := name != ""
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			bare = false
		}
	}
	if bare {
		return name
	}
	return strconv.Quote(name)
}
