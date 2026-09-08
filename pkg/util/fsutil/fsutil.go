// Package fsutil provides utilities for io/fs
package fsutil

import (
	"io/fs"
)

// MustSub returns fsys rooted at dir (see fs.Sub), panicking on error. The
// error is unreachable when dir names a compile-time //go:embed pattern root.
func MustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
