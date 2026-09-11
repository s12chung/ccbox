// Package slug slugifies paths for use in names (volume names, host dirs).
package slug

import "strings"

// Path slugifies a path: each "/" becomes a "-" (e.g. /Users/me/app → -Users-me-app).
func Path(p string) string { return strings.ReplaceAll(p, "/", "-") }
