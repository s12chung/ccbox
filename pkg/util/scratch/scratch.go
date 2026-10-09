// Package scratch shares host paths into the devbox: while a persistent path is
// missing, runs bind a seeded scratch copy, and the last out promotes its changes
// into the persistent path. Share drives the lifecycle over the
// Content and Sync axes — the file-vs-dir and symlink-vs-direct branches.
package scratch

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/errs"
	"github.com/s12chung/ccbox/pkg/util/flock"
	"github.com/s12chung/ccbox/pkg/util/fsutil"
	"github.com/s12chung/ccbox/pkg/util/klean"
	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/runid"
	"github.com/s12chung/ccbox/pkg/util/seed"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// Share drives the scratch lifecycle for Real across concurrent runs: the first
// run seeds it, latecomers bind it as-is, and the last out promotes its changes
// and cleans it up.
type Share struct {
	Name     string  // prefixes errors, e.g. "agents"
	RealPath string  // persistent path promoted into
	Content  Content // file vs dir
	Sync     Sync    // symlink vs direct
}

// Begin shares Real for the run, returning the host path to bind and a cleanup
// promoting the run's changes into Real. A "" path binds nothing: Real is reached
// via its own mount.
func (s Share) Begin() (string, func() error, error) {
	leave, err := s.multiflock().Join(s.verifyShared, s.initialShare, s.clean)
	if err != nil {
		// no holder entry: still tidy a crashed run's leftovers
		return "", s.clean, err
	}
	bind := s.Sync.ScratchBindPath(s.RealPath)
	if s.Sync.RealPresent(s.RealPath) {
		bind = s.Sync.RealBindPath(s.RealPath)
	}
	return bind, klean.SwallowErr(leave, flock.ErrNotLast), nil
}

// verifyShared verifies the share is present: the scratch's bind, or Real
// when the first holder bound it directly
func (s Share) verifyShared() error {
	if osutil.Missing(s.Sync.ScratchBindPath(s.RealPath)) && !s.Sync.RealPresent(s.RealPath) {
		return fmt.Errorf("%s: existing run's share missing: neither %s nor %s",
			s.Name, userdir.Tilde(s.Sync.ScratchBindPath(s.RealPath)), userdir.Tilde(s.RealPath))
	}
	return nil
}

// initialShare seeds the first holder's share: Real itself when present, the
// scratch otherwise
func (s Share) initialShare() error {
	if err := s.clean(); err != nil { // clean a crashed run's leftovers
		return err
	}
	if s.Sync.RealPresent(s.RealPath) {
		return nil
	}
	if err := s.Content.Seed(s.Sync.ScratchBindPath(s.RealPath)); err != nil {
		return err // no content to bind: the bind errors
	}
	return s.Sync.SeedSymlink(s.RealPath)
}

// clean promotes the scratch's changes into Real, then drops it wholesale. The
// symlink clears first: the write into Real must not follow our dangling symlink.
func (s Share) clean() error {
	if err := s.Sync.ClearSymlink(s.RealPath); err != nil {
		return err
	}
	bind := s.Sync.ScratchBindPath(s.RealPath)
	if osutil.Missing(bind) {
		return os.RemoveAll(s.Sync.ScratchPath(s.RealPath)) // a crashed run's leftovers
	}
	changed, err := s.Content.IsChanged(bind)
	if err != nil {
		return err
	}
	if changed {
		if err := s.Content.Promote(bind, s.RealPath); err != nil {
			return err
		}
	}
	return os.RemoveAll(s.Sync.ScratchPath(s.RealPath))
}

// multiflock tracks Real's existing runs by ScratchPath's rel path, re-rooted under
// the run registry, so a layout change never orphans existing runs' locks
func (s Share) multiflock() flock.MultiFlock {
	return flock.MultiFlock{Dir: runsDirFor(s.Sync.ScratchPath(s.RealPath))}
}

// Content is an interface splitting the File and Dir
// implementations: the scratch's contents, seeded into the bind and promoted into
// Real when the run changed them. Each step takes the bind — the file or root
// bound into the run.
type Content interface {
	Seed(bind string) error              // write the initial contents to the bind
	IsChanged(bind string) (bool, error) // whether the run changed the bind since Seed
	Promote(bind, realPath string) error // write the bind's changes into realPath
}

// File is a single-file scratch: Source's body at the bind, with the
// original for IsChanged kept outside the bind at +".orig".
type File struct {
	Source func() ([]byte, error) // the body the scratch binds
}

// Seed writes the source body to the bind and its original copy
func (f File) Seed(bind string) error {
	body, err := f.Source()
	if err != nil {
		return err
	}
	if err := osutil.SafeWriteFile(origFile(bind), body); err != nil {
		return err
	}
	return osutil.SafeWriteFile(bind, body)
}

// IsChanged diffs the bound file against the body it was bound from
func (f File) IsChanged(bind string) (bool, error) {
	bound, err := os.ReadFile(bind) // #nosec G304 -- the run's own bound copy
	if err != nil {
		return false, err
	}
	orig, err := os.ReadFile(origFile(bind)) // #nosec G304 -- the run's own original copy
	if errors.Is(err, fs.ErrNotExist) {
		orig, err = f.Source() // a past run's leftovers: diff against the source
	}
	if err != nil {
		return false, err
	}
	return !bytes.Equal(bound, orig), nil
}

// Promote overwrites realPath with the bound body, logging the set
func (f File) Promote(bind, realPath string) error {
	body, err := os.ReadFile(bind) // #nosec G304 -- the run's own bound copy
	if err != nil {
		return err
	}
	if err := osutil.SafeWriteFile(realPath, body); err != nil {
		return err
	}
	log.Infof("set %s", userdir.Tilde(realPath))
	return nil
}

func origFile(bind string) string { return bind + ".orig" }

// Dir is a dir scratch seeded from the embedded SeedFS; its bind is the
// scratch's whole root.
type Dir struct {
	SeedFS fs.FS
}

// Seed seeds the embedded tree onto the scratch
func (d Dir) Seed(bind string) error {
	_, err := seed.FS(d.SeedFS, bind)
	return errs.Swallow(err, seed.ErrNoChanges)
}

// IsChanged reports whether the run's tree differs from the embedded one
func (d Dir) IsChanged(bind string) (bool, error) {
	matches, err := fsutil.Matches(d.SeedFS, bind)
	return !matches, err
}

// Promote merges the run's tree into realPath, moving its differing copies aside
func (d Dir) Promote(bind, realPath string) error {
	asides, err := seed.Merge(bind, realPath, "run-"+runid.New())
	if err != nil {
		return err
	}
	for _, aside := range asides {
		log.Warnf("due to duplicate, run files saved as %s", userdir.Tilde(aside))
	}
	return nil
}

// Sync is an interface splitting the Direct and Symlink
// implementations: a temp scratch bound and cleared in each run (after handling any
// promoted changes to Real if needed). Real will take the place of the scratch when
// it exists.
type Sync interface {
	ScratchPath(realPath string) string     // the scratch root, cleaned after the run
	ScratchBindPath(realPath string) string // the scratch's host path bound while Real is missing: the scratch, or a file within
	RealBindPath(realPath string) string    // host path bound once Real exists; "" binds nothing
	SeedSymlink(realPath string) error      // seed a symlink at realPath to the container Mount
	ClearSymlink(realPath string) error     // clear SeedSymlink's symlink
	RealPresent(realPath string) bool       // realPath present as itself, not as SeedSymlink's symlink
}

// Direct creates the scratch and binds it directly.
type Direct struct{}

// ScratchPath is Real's rel path under userdir.Tmp()
func (Direct) ScratchPath(realPath string) string { return tmpDirFor(realPath) }

// ScratchBindPath is ScratchPath: the whole root binds
func (d Direct) ScratchBindPath(realPath string) string { return d.ScratchPath(realPath) }

// RealBindPath is Real itself
func (Direct) RealBindPath(realPath string) string { return realPath }

// SeedSymlink seeds no symlink
func (Direct) SeedSymlink(string) error { return nil }

// ClearSymlink clears nothing
func (Direct) ClearSymlink(string) error { return nil }

// RealPresent is Real's plain presence
func (Direct) RealPresent(realPath string) bool { return osutil.Present(realPath) }

// Symlink creates a scratch which is symlinked at realPath to the container
// Mount path.
type Symlink struct{ Mount string }

// ScratchPath is the ScratchBindPath's dir
func (l Symlink) ScratchPath(string) string { return filepath.Dir(l.hostPath()) }

// ScratchBindPath is the Mount's host path
func (l Symlink) ScratchBindPath(string) string { return l.hostPath() }

// hostPath is Mount re-rooted into userdir.Tmp():
// /home/ccbox/.ccbox/tmp/agents/<cli>/AGENTS.md → ~/.ccbox/tmp/agents/<cli>/AGENTS.md.
// A Mount outside tmpMount violates the layout invariant — unreachable via dmap's
// mounts — and panics.
func (l Symlink) hostPath() string {
	return filepath.Join(userdir.Tmp(), must.Get(pathTail(l.Mount, tmpMount)))
}

// RealBindPath binds nothing: Real is reached via its own bind elsewhere
func (Symlink) RealBindPath(string) string { return "" }

// SeedSymlink symlinks Real at the Mount
func (l Symlink) SeedSymlink(realPath string) error {
	return osutil.SafeSymlink(realPath, l.Mount)
}

// ClearSymlink drops our symlink only
func (l Symlink) ClearSymlink(realPath string) error {
	if osutil.IsSymlinkTo(realPath, l.Mount) {
		return os.Remove(realPath)
	}
	return nil
}

// RealPresent treats our own symlink — targeting Mount, dangling on the host by
// design — as missing, so a past run's leftover re-shares; foreign symlinks stay
// owned, untouched.
func (l Symlink) RealPresent(realPath string) bool {
	info, err := os.Lstat(realPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false // no doc of its own
	case err != nil:
		return true // unreadable (e.g. a directory): don't touch it
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return true
	}
	target, err := os.Readlink(realPath)
	return err != nil || target != l.Mount // unreadable/foreign: leave it be
}

// tmpMount is the container subtree whose rel paths re-root into userdir.Tmp()
const tmpMount = "/.ccbox/tmp"

// runsDirFor re-roots scratchPath's rel path under userdir.Tmp() into userdir.Runs():
// ~/.ccbox/tmp/persist/<slug> → ~/.ccbox/tmp/runs/persist/<slug>
func runsDirFor(scratchPath string) string {
	return filepath.Join(userdir.Runs(), must.Get(pathTail(scratchPath, userdir.Tmp())))
}

// tmpDirFor re-roots realPath's rel path under userdir.Dir() into userdir.Tmp():
// ~/.ccbox/persist/<slug> → ~/.ccbox/tmp/persist/<slug>
func tmpDirFor(realPath string) string {
	return filepath.Join(userdir.Tmp(), must.Get(pathTail(realPath, userdir.Dir())))
}

// pathTail returns p's rel path under parent; p outside parent violates the caller's
// layout invariant — unreachable via the built-in shares' wiring.
func pathTail(p, parent string) (string, error) {
	_, after, ok := strings.Cut(p, parent+string(filepath.Separator))
	if !ok {
		return "", fmt.Errorf("share: %s lacks %s", p, parent)
	}
	return after, nil
}
