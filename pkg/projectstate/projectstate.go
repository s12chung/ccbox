// Package projectstate seeds ccbox's own per-project state dir: laid onto the
// host per project and mounted at /home/ccbox/.ccbox/project in the devbox.
package projectstate

import (
	"embed"
	"io/fs"

	"github.com/s12chung/ccbox/pkg/util/embedfs"
)

//go:embed project-slug
var stateFS embed.FS

// SeedFS returns the embedded state tree, rooted at its content.
func SeedFS() fs.FS { return embedfs.MustSub(stateFS, "project-slug") }
