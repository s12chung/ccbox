// Package size measures filesystem sizes.
package size

import (
	"errors"
	"io/fs"
	"path/filepath"
)

// DirOver reports whether dir's files — symlinks not followed — total over limit.
// The walk stops at the first pass, so a dir far over limit costs no more than one
// just under it.
func DirOver(dir string, limit int64) (bool, error) {
	var total int64
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		if total > limit {
			return fs.SkipAll
		}
		return nil
	})
	// SkipAll is the early pass: the answer, not an error
	if errors.Is(err, fs.SkipAll) {
		err = nil
	}
	return total > limit, err
}
