// Package flock anchors exclusive kernel flock(2) locks on empty lockfiles: the kernel
// releases a lock when its holder dies, so the files need no stale handling. MultiFlock
// counts a shared resource's live holders with the same trick.
package flock

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

// ErrHeld reports the lock is held elsewhere
var ErrHeld = errors.New("lock: already held")

// Lock is an exclusive flock on an empty lockfile
type Lock struct{ f *os.File }

// Ex takes a blocking exclusive lock at path, waiting out a holder's death-released lock
func Ex(path string) (*Lock, error) {
	return take(path, syscall.LOCK_EX)
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

func take(path string, how int) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(path), ioutil.Dir); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, ioutil.File) // #nosec G304 -- path is the lock's own file
	if err != nil {
		return nil, err
	}
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
