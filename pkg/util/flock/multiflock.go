package flock

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

const (
	// entryPrefix marks holder entry files, whose names are mere handles: an entry
	// exists while its flock is held
	entryPrefix = "holder-"

	// lockFileName is never removed: an immortal lockfile avoids the unlink+flock
	// inode-reuse pitfall
	lockFileName = ".lock"
)

// ErrNotLast reports that existing holders remain: the last one out swallows it to
// skip its teardown
var ErrNotLast = errors.New("flock: an existing holder remains")

// MultiFlock tracks a shared resource's existing holders in Dir: one flock-held entry per
// holder, so the kernel — not stored state — carries the count, and a dead holder's
// entry is swept once its lock is gone.
type MultiFlock struct{ Dir string }

// Join flocks the holder's entry in Dir and runs one callback under the dir lock: fn
// creates the resource for its first holder, verifyExisting verifies it for
// latecomers. It returns the holder's leave, which runs teardown for the last one out.
func (m MultiFlock) Join(verifyExisting, fn, teardown func() error) (func() error, error) {
	release, existing, err := m.Clean()
	if err != nil {
		return nil, err
	}
	defer release()

	if existing > 0 { // an existing holder holds the resource: verify it, don't create it
		fn = verifyExisting
	}
	// the callback precedes the registration: a failed join leaves nothing behind
	if err := fn(); err != nil {
		return nil, err
	}
	entry, err := newEntry(m.Dir)
	if err != nil {
		return nil, err
	}
	return func() error { return m.leave(entry, teardown) }, nil
}

// Clean takes the dir lock and sweeps the dead entries, returning the lock's release
// and the existing holders' count
func (m MultiFlock) Clean() (func(), int, error) {
	dirLock, err := Ex(m.lockPath())
	if err != nil {
		return nil, 0, err
	}
	release := func() { log.Defer("release dir lock", dirLock.Release) }
	existing, err := m.sweep()
	if err != nil {
		release()
		return nil, 0, err
	}
	return release, existing, nil
}

// leave sweeps the dead ones and drops the calling holder's entry; an existing holder
// remaining reports ErrNotLast and skips teardown, the last one out runs teardown first
func (m MultiFlock) leave(e *entry, teardown func() error) error {
	release, existing, err := m.Clean() // the sweep counts the calling holder's own entry
	if err != nil {
		return err
	}
	defer release()

	if existing > 1 { // the count includes the calling holder: the remainder is the others
		if err := e.drop(); err != nil {
			return err
		}
		return ErrNotLast
	}
	// teardown runs under the dir lock, before the entry drops: a join blocked on the
	// lock then sees the resource fully up or fully torn down, never half
	var terr error
	if teardown != nil {
		terr = teardown()
	}
	// the entry drops regardless: a failed teardown must not hold the count hostage
	return errors.Join(terr, e.drop())
}

// sweep counts the existing entries and removes the dead ones. It runs under the dir
// lock, so every entry is fully registered (its flock held) or abandoned (its holder
// died) — no half-registered state to mistake for either.
func (m MultiFlock) sweep() (int, error) {
	entries, err := os.ReadDir(m.Dir)
	if err != nil {
		return 0, err
	}
	existing := 0
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), entryPrefix) || e.IsDir() {
			continue
		}
		p := filepath.Join(m.Dir, e.Name())
		switch l, err := TryEx(p); {
		case errors.Is(err, ErrHeld):
			existing++
		case err != nil:
			return 0, err
		default:
			if err := errors.Join(l.Release(), os.Remove(p)); err != nil {
				return 0, err
			}
		}
	}
	return existing, nil
}

func (m MultiFlock) lockPath() string { return filepath.Join(m.Dir, lockFileName) }

// entry is one holder's registration: an empty file flock-held for the holder's life
type entry struct {
	path string
	l    *Lock
}

// newEntry flocks the fresh entry file under the dir lock: no sweep can run between
// the create and the lock
func newEntry(dir string) (*entry, error) {
	f, err := os.CreateTemp(dir, entryPrefix) // unique name, never a PID
	if err != nil {
		return nil, err
	}
	p := f.Name()
	if err := f.Close(); err != nil {
		return nil, err
	}
	l, err := TryEx(p)
	if err != nil {
		return nil, errors.Join(err, os.Remove(p))
	}
	return &entry{path: p, l: l}, nil
}

func (e *entry) drop() error { return errors.Join(os.Remove(e.path), e.l.Release()) }
