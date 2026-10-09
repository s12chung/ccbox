// Package installutil holds utils for versioned installs under the
// clis and apps volumes: staging renames, the current-symlink flip, and pruning.
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

// CurrentVersion returns the version the current symlink at link points at, ""
// when unset.
func CurrentVersion(link string) string {
	dest, err := os.Readlink(link)
	if err != nil {
		return ""
	}
	return filepath.Base(dest)
}

// Prune removes every entry under dir except current and keep — including
// .tmp leftovers from a crashed install.
func Prune(dir, keep string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == keep || entry.Name() == "current" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}
