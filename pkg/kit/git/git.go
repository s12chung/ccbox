// Package git resolves host-side git paths for mounting into the devbox.
package git

import (
	"os"
	"path/filepath"

	"github.com/s12chung/ccbox/pkg/userdir"
)

// XDGConfigDir returns the host XDG git config dir (~/.config/git, honoring XDG_CONFIG_HOME).
// The dir need not exist on disk — the caller present-checks.
func XDGConfigDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(userdir.MustHome(), ".config")
	}
	return filepath.Join(base, "git")
}
