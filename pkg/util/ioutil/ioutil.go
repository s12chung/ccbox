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

// Missing returns whether path does not exist
func Missing(path string) bool {
	_, err := os.Stat(path)
	return errors.Is(err, fs.ErrNotExist)
}

// Present returns whether path exists
func Present(path string) bool { return !Missing(path) }

// SafeWriteFile writes body at path with File perms, creating its parent dir when missing
func SafeWriteFile(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), Dir); err != nil {
		return err
	}
	return os.WriteFile(path, body, File)
}

// DirsPresent returns the entries of dirs that exist as directories under src.
func DirsPresent(src string, dirs []string) []string {
	var out []string
	for _, d := range dirs {
		if info, err := os.Stat(filepath.Join(src, d)); err == nil && info.IsDir() {
			out = append(out, d)
		}
	}
	return out
}
