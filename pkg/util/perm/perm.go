// Package perm names the file/directory permission bits used across ccbox, so
// call sites read intent instead of bare octal.
package perm

import "os"

// Modes for the files and directories ccbox writes.
const (
	Dir      os.FileMode = 0o755 // directories
	File     os.FileMode = 0o644 // regular files
	ExecFile os.FileMode = 0o755 // executable files (e.g. shell scripts)
)
