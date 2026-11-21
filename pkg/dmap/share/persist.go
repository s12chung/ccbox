package share

import (
	"path/filepath"

	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/scratch"
	"github.com/s12chung/ccbox/pkg/util/slug"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

// PersistDir returns the Binder for the project's persist dir.
func PersistDir(projectDir string) scratch.Binder {
	return scratch.Binder{
		Share: scratch.Share{
			Name:     "persist",
			RealPath: filepath.Join(userdir.Dir(), "persist", slug.Path(projectDir)),
			Content:  scratch.Dir{},
			Sync:     scratch.Direct{},
		},
		BindPath: pkginfo.ContainerHome + "/.ccbox/persist",
	}
}
