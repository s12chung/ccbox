// Package slug slugifies paths for use in names (volume names, host dirs).
package slug

import "strings"

// Path slugifies a path: "-" doubles to "--", then "/" becomes "-" (e.g.
// /Users/me/a-b → -Users-me-a--b)
func Path(p string) string {
	return strings.ReplaceAll(strings.ReplaceAll(p, "-", "--"), "/", "-")
}
