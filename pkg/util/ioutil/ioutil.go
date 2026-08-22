// Package ioutil contains utils for io
package ioutil

import (
	"os"
	"path/filepath"
)

// Modes for the files and directories ccbox writes.
const (
	Dir      os.FileMode = 0o755 // directories
	File     os.FileMode = 0o644 // regular files
	ExecFile os.FileMode = 0o755 // executable files (e.g. shell scripts)
)

// DirsPresentInSrc returns the entries of dirs that exist as directories under workspaceDir.
func DirsPresentInSrc(src string, dirs []string) []string {
	var out []string
	for _, d := range dirs {
		if info, err := os.Stat(filepath.Join(src, d)); err == nil && info.IsDir() {
			out = append(out, d)
		}
	}
	return out
}
