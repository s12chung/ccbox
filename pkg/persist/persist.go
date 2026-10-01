// Package persist seeds the per-project state dir persisted across runs: mounted at
// /home/ccbox/.ccbox/persist in the devbox, kept on the host per project.
package persist

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/flock"
	"github.com/s12chung/ccbox/pkg/util/fsutil"
	"github.com/s12chung/ccbox/pkg/util/fsync"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/klean"
	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/runid"
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
// copy, and the run's changes are promoted into the Dir.
type Share struct{ ProjectDir string }

// Begin binds the persist mount: an existing Dir directly; a missing one, a seeded
// scratch copy whose cleanup promotes changes into the Dir. Concurrent runs share the
// one scratch: the first run seeds it, latecomers bind it as-is, the last out cleans it up.
func (s Share) Begin() (string, func() error, error) {
	leave, err := s.multiflock().Join(s.verifyShared, s.initialShare)
	clean := klean.SwallowErr(klean.NewQueue(leave, s.clean).Run, flock.ErrNotLast)
	if err != nil {
		return "", clean, err
	}
	if ioutil.Present(s.realDir()) {
		return s.realDir(), clean, nil
	}
	return s.scratch(), clean, nil
}

// verifyShared verifies the share a live entry implies is present: the scratch, or the
// realDir when the first holder bound it directly
func (s Share) verifyShared() error {
	if ioutil.Missing(s.scratch()) && ioutil.Missing(s.realDir()) {
		return fmt.Errorf("persist: live run's share missing: neither %s nor %s", userdir.Tilde(s.scratch()), userdir.Tilde(s.realDir()))
	}
	return nil
}

func (s Share) initialShare() error {
	if err := s.clean(); err != nil { // clean a crashed run's leftover scratch
		return err
	}
	if ioutil.Present(s.realDir()) { // the Dir is the share: bound directly, no scratch seeded
		return nil
	}
	_, err := fsync.Seed(SeedFS(), s.scratch())
	if errors.Is(err, fsync.ErrNoChanges) {
		return nil
	}
	return err
}

// clean promotes the run's changes into the Dir, then drops the scratch
func (s Share) clean() error {
	scratch := s.scratch()
	if ioutil.Missing(scratch) {
		return nil
	}
	matches, err := fsutil.Matches(SeedFS(), scratch)
	if err != nil {
		return err
	}
	if matches {
		return os.RemoveAll(scratch) // unchanged: an untouched folder leaves nothing on the host
	}
	if err := s.promote(); err != nil {
		return err
	}
	return os.RemoveAll(scratch)
}

// promote merges the scratch into the Dir, moving the Dir's differing copies
// aside for the run's changes
func (s Share) promote() error {
	asides, err := fsync.Merge(s.scratch(), s.realDir(), "run-"+runid.New())
	if err != nil {
		return err
	}
	for _, aside := range asides {
		log.Warnf("due to duplicate, run persist files saved as %s", userdir.Tilde(aside))
	}
	return nil
}

// realDir is the project's Dir: ~/.ccbox/persist/<slug>
func (s Share) realDir() string { return Dir(s.ProjectDir) }

// multiflock tracks the project's live runs: ~/.ccbox/tmp/runs/persist/<slug>
func (s Share) multiflock() flock.MultiFlock {
	return flock.MultiFlock{Dir: filepath.Join(userdir.Runs(), "persist", slug.Path(s.ProjectDir))}
}

// scratch is the seeded scratch copy of dir: ~/.ccbox/tmp/persist/<slug>
func (s Share) scratch() string {
	return filepath.Join(userdir.Tmp(), "persist", slug.Path(s.ProjectDir))
}
