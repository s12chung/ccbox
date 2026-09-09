// Package lock guards volume-shared installs across containers. The flock is
// kernel-held — released when the holding container dies — so the persisted
// lock file needs no stale handling.
package lock

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
)

const (
	dirMode  os.FileMode = 0o755
	fileMode os.FileMode = 0o644
)

// Do runs fn under an exclusive flock at lockPath. When the lock is held, skip
// decides: true skips fn; false waits for the lock, then runs fn.
func Do(lockPath string, fn func() error, skip func() bool) error {
	if err := os.MkdirAll(filepath.Dir(lockPath), dirMode); err != nil {
		return err
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, fileMode) // #nosec G304 -- lockPath is the clis volume's own lock file
	if err != nil {
		return err
	}
	defer log.Defer("close lock file", f.Close)
	fd := int(f.Fd())

	// take the lock, declining rather than waiting it out
	if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return err
		}
		if skip() {
			return nil
		}
		// wait out the holder, then take the lock
		if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
			return err
		}
	}
	// release the lock once fn is done
	defer log.Defer("unlock install lock", func() error { return syscall.Flock(fd, syscall.LOCK_UN) })
	return fn()
}
