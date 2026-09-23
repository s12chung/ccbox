// Package persist seeds the per-project state dir persisted across runs: mounted at
// /home/ccbox/.ccbox/persist in the devbox, kept on the host per project.
package persist

import (
	"embed"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/fsutil"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/seed"
	"github.com/s12chung/ccbox/pkg/util/slug"
)

//go:embed seed
var stateFS embed.FS

// SeedFS returns the embedded state tree, rooted at its content.
func SeedFS() fs.FS { return must.Get(fs.Sub(stateFS, "seed")) }

// Dir is the host state dir persisted for a project: ~/.ccbox/persist/<slug>
func Dir(projectDir string) string {
	return filepath.Join(userdir.Dir(), "persist", slug.Path(projectDir))
}

// Share persists the project's Dir across runs: the mount is bound via a scratch
// copy, and the run's changes are settled into the Dir.
type Share struct{ ProjectDir string }

var noop = func() error { return nil }

// Begin binds the persist mount: an existing Dir binds directly; a missing one binds
// a seeded scratch copy, whose cleanup settles changes into the Dir.
func (s Share) Begin() (string, func() error, error) {
	if err := s.clean(); err != nil { // clean a crashed run's leftover scratch
		return "", noop, err
	}
	if ioutil.Present(s.dir()) {
		return s.dir(), noop, nil
	}
	if _, err := seed.Tree(SeedFS(), s.scratch()); err != nil && !errors.Is(err, seed.ErrNoChanges) {
		return "", noop, err
	}
	return s.scratch(), s.clean, nil
}

// clean settles the run's changes into a missing Dir, then drops the scratch
func (s Share) clean() error {
	scratch := s.scratch()
	if ioutil.Missing(scratch) {
		return nil
	}
	matches, err := fsutil.Matches(SeedFS(), scratch)
	if err != nil {
		return err
	}
	if !matches {
		return s.promote()
	}
	return os.RemoveAll(scratch) // unchanged: an untouched folder leaves nothing on the host
}

// promote settles the scratch into Dir, logging the set
func (s Share) promote() error {
	if err := os.MkdirAll(filepath.Dir(s.dir()), ioutil.Dir); err != nil {
		return err
	}
	if err := os.Rename(s.scratch(), s.dir()); err != nil {
		return err
	}
	log.Infof("set %s", userdir.Tilde(s.dir()))
	return nil
}

// dir is the project's Dir: ~/.ccbox/persist/<slug>
func (s Share) dir() string { return Dir(s.ProjectDir) }

// scratch is the seeded scratch copy of dir: ~/.ccbox/tmp/persist/<slug>
func (s Share) scratch() string {
	return filepath.Join(userdir.Tmp(), "persist", slug.Path(s.ProjectDir))
}
