package install

import (
	"os"
	"path/filepath"
)

// dirMode is the mode dirs under the clis root carry.
const dirMode os.FileMode = 0o755

func safeMkdir(p string) error {
	if err := os.RemoveAll(p); err != nil {
		return err
	}
	if err := os.MkdirAll(p, dirMode); err != nil {
		return err
	}
	return nil
}

func safeMv(src, dest string) error {
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := os.Rename(src, dest); err != nil {
		return err
	}
	return nil
}

// replaceSymlink points dest at src atomically: create under a temp name,
// then rename over the link.
func replaceSymlink(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), dirMode); err != nil {
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

// prune removes every entry under dir except current and the active version —
// including .tmp leftovers from a crashed install.
func prune(dir, version string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == "current" || entry.Name() == version {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}
