// Package installutil holds utils for versioned installs under the
// clis and apps volumes: staging renames, symlinks, and pruning.
package installutil

import (
	"os"
	"path/filepath"
)

// DirMode is the mode staged dirs carry.
const DirMode os.FileMode = 0o755

// SafeMkdir recreates p empty, over any earlier tree.
func SafeMkdir(p string) error {
	if err := os.RemoveAll(p); err != nil {
		return err
	}
	return os.MkdirAll(p, DirMode)
}

// SafeMv moves src onto dest, replacing whatever dest already holds.
func SafeMv(src, dest string) error {
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	return os.Rename(src, dest)
}

// ReplaceSymlink points dest at src atomically: create under a temp name,
// then rename over the link.
func ReplaceSymlink(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), DirMode); err != nil {
		return err
	}
	tmp := dest + ".tmp"
	if err := os.Remove(tmp); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Symlink(src, tmp); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

// Prune removes every entry under dir except keep and the versions held
// reports — including .tmp leftovers from a crashed install.
func Prune(dir, keep string, held func(version string) bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == keep || held(name) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}
