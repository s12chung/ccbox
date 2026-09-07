// Package ioutil contains utils for io
package ioutil

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Modes for the files and directories ccbox writes.
const (
	Dir      os.FileMode = 0o755 // directories
	File     os.FileMode = 0o644 // regular files
	ExecFile os.FileMode = 0o755 // executable files (e.g. shell scripts)
)

// Missing reports whether path does not exist
func Missing(path string) bool {
	_, err := os.Stat(path)
	return errors.Is(err, fs.ErrNotExist)
}

// PathsPresent returns the entries of paths that exist under src.
func PathsPresent(src string, paths []string) []string {
	var out []string
	for _, p := range paths {
		if _, err := os.Stat(filepath.Join(src, p)); err == nil {
			out = append(out, p)
		}
	}
	return out
}
