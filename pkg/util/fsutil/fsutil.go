// Package fsutil provides utilities for io/fs
package fsutil

import (
	"io/fs"

	"github.com/s12chung/ccbox/pkg/util/must"
)

// MustSub returns fsys rooted at dir (see fs.Sub), panicking on error. The
// error is unreachable when dir names a compile-time //go:embed pattern root.
func MustSub(fsys fs.FS, dir string) fs.FS { return must.Get(fs.Sub(fsys, dir)) }
