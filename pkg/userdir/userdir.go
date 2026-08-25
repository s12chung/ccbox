// Package userdir resolves ccbox's per-user directory for configs and persistent storage.
package userdir

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
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
