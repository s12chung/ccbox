// Package userdir resolves ccbox's per-user directory for configs and persistent storage.
package userdir

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var dir string

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("userdir: %v", err))
	}
	dir = filepath.Join(home, ".ccbox")
}

// Dir is ccbox's per-user directory: ~/.ccbox.
func Dir() string { return dir }

// ConfigDir is ccbox's per-user config directory: ~/.ccbox/config
func ConfigDir() string { return path.Join(dir, "config") }

// Tilde abbreviates the home dir in p as ~/... for display. Paths outside home (and
// a home that can't be resolved) come back unchanged.
func Tilde(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	sep := string(filepath.Separator)
	switch {
	case p == home:
		return "~"
	case strings.HasPrefix(p, home+sep):
		return "~" + sep + strings.TrimPrefix(p, home+sep)
	}
	return p
}
