// Package git resolves host-side git paths for mounting into the devbox.
package git

import (
	"os"
	"path/filepath"
)

// XDGConfigDir returns the host XDG git config dir (~/.config/git, honoring XDG_CONFIG_HOME) if it
// exists as a directory, else "".
func XDGConfigDir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	dir := filepath.Join(base, "git")
	switch info, err := os.Stat(dir); { // #nosec G703 -- dir is the user's own XDG git config path
	case err == nil && info.IsDir():
		return dir, nil
	case err == nil || os.IsNotExist(err): // a non-dir or absent path → skip, not an error
		return "", nil
	default:
		return "", err
	}
}
