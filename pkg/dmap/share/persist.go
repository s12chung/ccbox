package share

import (
	"embed"
	"io/fs"
	"path/filepath"

	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/scratch"
	"github.com/s12chung/ccbox/pkg/util/slug"
)

//go:embed seed
var stateFS embed.FS

// seedFS returns the embedded state tree, rooted at its content.
func seedFS() fs.FS { return must.Get(fs.Sub(stateFS, "seed")) }

// PersistDir wires the share driver for the project's persist dir, seeded from the
// embedded state tree.
func PersistDir(projectDir string) scratch.Binder {
	return scratch.Binder{
		Share: scratch.Share{
			Name:     "persist",
			RealPath: filepath.Join(userdir.Dir(), "persist", slug.Path(projectDir)),
			Content:  scratch.Dir{SeedFS: seedFS()},
			Sync:     scratch.Direct{},
		},
		BindPath: projectcfg.ContainerHome + "/.ccbox/persist",
	}
}
