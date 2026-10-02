package flock

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/s12chung/ccbox/ccboxtools/pkg/util/log"
)

const (
	// entryPrefix marks holder entry files, whose names are mere handles: the flocks
	// carry the liveness
	entryPrefix = "holder-"

	// lockFileName is never removed: an immortal lockfile avoids the unlink+flock
	// inode-reuse pitfall
	lockFileName = ".lock"
)

// ErrNotLast reports that the leaving holder was not the last live one: the
// last one out swallows it to skip its clean
var ErrNotLast = errors.New("flock: a live holder remains")

// MultiFlock tracks a shared resource's live holders in Dir: one flock-held entry per
// holder, so the kernel — not stored state — carries the count, and a dead holder's
// entry is swept once its lock is gone.
type MultiFlock struct{ Dir string }

// Join flocks the holder's entry in Dir and runs one callback under the dir lock: fn
// creates the resource for its first holder, verifyExisting verifies it for
// latecomers. The callback precedes the registration, so a failed join leaves
// nothing behind. It returns the holder's leave.
func (m MultiFlock) Join(verifyExisting, fn func() error) (func() error, error) {
	release, live, err := m.Clean()
	if err != nil {
		return nil, err
	}
	defer release()

	if live > 0 {
		fn = verifyExisting
	}
	if err := fn(); err != nil {
		return nil, err
	}
	entry, err := newEntry(m.Dir)
	if err != nil {
		return nil, err
	}
	return func() error { return m.leave(entry) }, nil
}

// Clean takes the dir lock and sweeps the dead entries, returning the lock's release
// and the live holders' count
func (m MultiFlock) Clean() (func(), int, error) {
	dirLock, err := Ex(m.lockPath())
	if err != nil {
		return nil, 0, err
	}
	release := func() { log.Defer("release dir lock", dirLock.Release) }
	live, err := m.sweep()
	if err != nil {
		release()
		return nil, 0, err
	}
	return release, live, nil
}

// leave sweeps the dead ones and drops the holder's entry; a live holder remaining
// reports ErrNotLast
func (m MultiFlock) leave(e *entry) error {
	release, live, err := m.Clean() // the sweep counts this holder's own live entry
	if err != nil {
		return err
	}
	defer release()

	if err := e.drop(); err != nil {
		return err
	}
	if live > 1 { // the count includes this holder: the remainder is the others
		return ErrNotLast
	}
	return nil
}

// sweep counts the live entries and removes the dead ones. It runs under the dir
// lock, so every entry is fully registered (its flock held) or abandoned (its holder
// died) — no half-registered state to mistake for either.
func (m MultiFlock) sweep() (int, error) {
	entries, err := os.ReadDir(m.Dir)
	if err != nil {
		return 0, err
	}
	live := 0
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), entryPrefix) || e.IsDir() {
			continue
		}
		p := filepath.Join(m.Dir, e.Name())
		switch l, err := TryEx(p); {
		case errors.Is(err, ErrHeld):
			live++
		case err != nil:
			return 0, err
		default:
			if err := errors.Join(l.Release(), os.Remove(p)); err != nil {
				return 0, err
			}
		}
	}
	return live, nil
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
