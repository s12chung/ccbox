// Package persist seeds the per-project state dir persisted across runs: laid onto
// the host per project and mounted at /home/ccbox/.ccbox/persist in the devbox.
package persist

import (
	"embed"
	"io/fs"

	"github.com/s12chung/ccbox/pkg/util/must"
)

//go:embed seed
var stateFS embed.FS

// SeedFS returns the embedded state tree, rooted at its content.
func SeedFS() fs.FS { return must.Get(fs.Sub(stateFS, "seed")) }
