// Package seed lays config content onto host paths: whole trees, backing up any
// files it would overwrite, and single files that never touch existing ones.
package seed

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

// ErrExists reports that File's path already exists: an existing file is never
// touched, so there is nothing to seed.
var ErrExists = errors.New("seed: file already exists")

// File seeds path with body when absent, creating its parent dir. An existing file
// is never touched: the returned error wraps ErrExists, carrying the path.
func File(path, body string) error {
	switch _, err := os.Stat(path); {
	case err == nil:
		return fmt.Errorf("%s: %w", path, ErrExists)
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), ioutil.Dir); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), ioutil.File)
}

// ErrNoChanges reports that Tree made no changes: every destination already
// matched its source, so nothing was written or backed up.
var ErrNoChanges = errors.New("seed: all files identical")

// Tree seeds fsys's tree onto destDir (creating it), preserving the tree.
// A destination already matching the source is left untouched. Other existing
// destination files are backed up to <base>.old<ext> before being overwritten; the
// backed-up paths are returned. A pre-existing backup is never clobbered — it's a hard error.
// When every file is left untouched, ErrNoChanges is returned.
func Tree(fsys fs.FS, destDir string) ([]string, error) {
	var renamed []string
	var changed int
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}

		dest := filepath.Join(destDir, p)
		if err := os.MkdirAll(filepath.Dir(dest), ioutil.Dir); err != nil {
			return err
		}
		if existing, err := os.ReadFile(dest); err == nil { // #nosec G304 -- dest is the seeded tree's own path
			if bytes.Equal(existing, body) {
				return nil
			}
			backup := backupPath(dest)
			if _, err := os.Stat(backup); err == nil {
				return fmt.Errorf("seed: backup already exists, refusing to overwrite: %s", backup)
			} else if !os.IsNotExist(err) {
				return err
			}
			if err := os.Rename(dest, backup); err != nil {
				return err
			}
			renamed = append(renamed, backup)
		}
		changed++
		return os.WriteFile(dest, body, fileMode(dest))
	})
	if err == nil && changed == 0 {
		err = ErrNoChanges
	}
	return renamed, err
}

// backupPath inserts ".old" before the extension: foo/bar.json -> foo/bar.old.json.
func backupPath(p string) string {
	ext := filepath.Ext(p)
	return strings.TrimSuffix(p, ext) + ".old" + ext
}

// fileMode makes shell scripts executable; everything else is a regular file.
func fileMode(p string) os.FileMode {
	if filepath.Ext(p) == ".sh" {
		return ioutil.ExecFile
	}
	return ioutil.File
}
