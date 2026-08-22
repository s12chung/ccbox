// Package projectstate seeds ccbox's own per-project state dir: laid onto the
// host per project and mounted at /home/ccbox/.ccbox/project in the devbox.
package projectstate

import (
	"embed"
	"io/fs"
)

//go:embed project-slug
var stateFS embed.FS

// SeedFS returns the embedded state tree, rooted at its content.
func SeedFS() fs.FS {
	sub, err := fs.Sub(stateFS, "project-slug")
	if err != nil {
		panic(err) // unreachable: the //go:embed pattern above guarantees project-slug exists
	}
	return sub
}
