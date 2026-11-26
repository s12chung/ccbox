// Package flock anchors kernel flock(2) locks on empty lockfiles — or on the
// existing dir a hold guards: the kernel releases a lock when its holder dies, so
// nothing needs stale handling. MultiFlock counts a shared resource's existing
// holders with the same trick.
package flock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

const (
	dirMode  os.FileMode = 0o755
	fileMode os.FileMode = 0o644

	// waitPoll spaces the lock wait's retries: the holders are container
	// lifecycle operations, so the poll trades promptness for no blocking syscall
	waitPoll = 50 * time.Millisecond
)

// ErrHeld reports the lock is held elsewhere
var ErrHeld = errors.New("lock: already held")

// Lock is a kernel flock on an open file: a lockfile, or the dir a hold guards
type Lock struct{ f *os.File }

// Ex takes a blocking exclusive lock at path, waiting out a holder's death-released lock
func Ex(path string) (*Lock, error) { return take(path, syscall.LOCK_EX) }

// Hold takes a shared lock on the existing path, coexisting with its other
// holders; the returned func lets go
func Hold(path string) (func() error, error) {
	l, err := takeExisting(path, syscall.LOCK_SH)
	if err != nil {
		return nil, err
	}
	return l.Release, nil
}

// TryEx takes a non-blocking exclusive lock at path: ErrHeld when held elsewhere
func TryEx(path string) (*Lock, error) {
	l, err := take(path, syscall.LOCK_EX|syscall.LOCK_NB)
	switch {
	case err == nil:
		return l, nil
	case errors.Is(err, syscall.EWOULDBLOCK):
		return nil, ErrHeld
	default:
		return nil, err
	}
}

// Held reports whether the lock at path is held elsewhere: held means a live
// holder, and a missing path is never held — the probe creates none
func Held(path string) bool {
	l, err := takeExisting(path, syscall.LOCK_EX|syscall.LOCK_NB)
	switch {
	case err == nil:
		log.WarnErr("release held probe", l.Release())
		return false // the probe itself took the lock: nobody holds it
	case errors.Is(err, syscall.EWOULDBLOCK):
		return true
	default:
		log.WarnErr("probe lock", err)
		return false
	}
}

func take(path string, how int) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, fileMode) // #nosec G304 -- path is the lock's own file
	if err != nil {
		return nil, err
	}
	return flockFile(f, how)
}

// takeExisting takes how at the existing path, creating nothing
func takeExisting(path string, how int) (*Lock, error) {
	f, err := os.Open(path) // #nosec G304 -- the caller's own path, not user input
	if err != nil {
		return nil, err
	}
	return flockFile(f, how)
}

func flockFile(f *os.File, how int) (*Lock, error) {
	if err := syscall.Flock(int(f.Fd()), how); err != nil {
		return nil, errors.Join(err, f.Close())
	}
	return &Lock{f: f}, nil
}

// Release unlocks and closes the lock's file
func (l *Lock) Release() error {
	err := syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	return errors.Join(err, l.f.Close())
}

// Do runs fn under an exclusive flock at lockPath. A held lock skips fn when
// skip returns true; skip unset or false waits the holder out — the ctx's
// cancel ends the wait — then runs fn.
func Do(ctx context.Context, lockPath string, fn func() error, skip func() bool) error {
	l, err := TryEx(lockPath)
	switch {
	case err == nil:
	case errors.Is(err, ErrHeld):
		if skip != nil && skip() {
			return nil
		}
		// wait out the holder, then take the lock
		if l, err = waitEx(ctx, lockPath); err != nil {
			return err
		}
	default:
		return err
	}
	defer log.Defer("release lock", l.Release)
	return fn()
}

// waitEx waits out the holder and takes the lock — LOCK_NB retries instead of
// one blocking Flock, so the ctx's cancel ends the wait.
func waitEx(ctx context.Context, path string) (*Lock, error) {
	for {
		l, err := TryEx(path)
		switch {
		case err == nil:
			return l, nil
		case !errors.Is(err, ErrHeld):
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("lock %s: %w", path, ctx.Err())
		case <-time.After(waitPoll):
		}
	}
}
