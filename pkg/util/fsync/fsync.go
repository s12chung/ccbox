// Package fsync seeds config content onto host paths: whole trees, backing up any
// files it would overwrite, and single files that never touch existing ones.
package fsync

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/s12chung/ccbox/pkg/util/errs"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

// ErrExists reports that File's path already exists: an existing file is never
// touched, so there is nothing to seed.
var ErrExists = errors.New("fsync: file already exists")

// File seeds path with body when absent, creating its parent dir. An existing file
// is never touched: the returned error wraps ErrExists, carrying the path.
func File(path string, body []byte) error {
	switch _, err := os.Stat(path); {
	case err == nil:
		return fmt.Errorf("%s: %w", path, ErrExists)
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), ioutil.Dir); err != nil {
		return err
	}
	return os.WriteFile(path, body, ioutil.File)
}

// SafeFile is File with ErrExists swallowed: an existing file — a mid-race
// appearance included — is a no-op, not an error; it is never touched.
func SafeFile(path string, body []byte) error { return errs.Swallow(File(path, body), ErrExists) }

// ErrNoChanges reports that Seed made no changes: every destination already
// matched its source, so nothing was written or backed up.
var ErrNoChanges = errors.New("fsync: all files identical")

// Seed seeds fsys's tree onto destDir (creating it), preserving the tree.
// A destination already matching the source is left untouched. Other existing
// destination files are moved aside to <base>.old<ext> before being overwritten; the
// aside paths are returned. A pre-existing aside is never clobbered — it's a hard error.
// When every file is left untouched, ErrNoChanges is returned.
func Seed(fsys fs.FS, destDir string) ([]string, error) {
	asides, changed, err := sync(fsys, destDir, "old")
	if err == nil && !changed {
		return asides, ErrNoChanges
	}
	return asides, err
}

// Merge moves srcDir's changes into dstDir, creating dstDir when missing: files
// dstDir lacks are written in as-is, identical files are dropped, and differing
// files land in dstDir with dstDir's copies moved aside as <base>.<infix>.<ext> —
// one merge's asides sharing one infix. The aside paths are returned.
func Merge(srcDir, dstDir, infix string) ([]string, error) {
	switch _, err := os.Stat(dstDir); {
	case errors.Is(err, fs.ErrNotExist):
		if err := os.MkdirAll(filepath.Dir(dstDir), ioutil.Dir); err != nil {
			return nil, err
		}
		return nil, os.Rename(srcDir, dstDir)
	case err != nil:
		return nil, err
	}
	asides, _, err := sync(os.DirFS(srcDir), dstDir, infix)
	return asides, err
}

// sync writes fsys's tree onto destDir (creating it), preserving the tree: a
// destination matching its source is left untouched; every other file is written
// over, its differing copy moved aside to <base>.<infix><ext> first — a pre-existing
// aside is never clobbered, it's a hard error. Empty dirs are created as-is. The second
// return reports that any file was written.
func sync(fsys fs.FS, destDir, infix string) ([]string, bool, error) {
	var asides []string
	var changed bool
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(destDir, p), ioutil.Dir)
		}
		body, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}

		dest := filepath.Join(destDir, p)
		if err := os.MkdirAll(filepath.Dir(dest), ioutil.Dir); err != nil {
			return err
		}
		if existing, err := os.ReadFile(dest); err == nil { // #nosec G304 -- dest is the synced tree's own path
			if bytes.Equal(existing, body) {
				return nil
			}
			aside, err := asideDest(dest, infix)
			if err != nil {
				return err
			}
			asides = append(asides, aside)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		changed = true
		return os.WriteFile(dest, body, fileMode(dest))
	})
	return asides, changed, err
}

// asideDest moves dest aside to <base>.<infix><ext>, refusing to clobber a
// pre-existing aside
func asideDest(dest, infix string) (string, error) {
	ext := filepath.Ext(dest)
	aside := strings.TrimSuffix(dest, ext) + "." + infix + ext
	switch _, err := os.Stat(aside); {
	case err == nil:
		return "", fmt.Errorf("fsync: aside already exists, refusing to overwrite: %s", aside)
	case !errors.Is(err, fs.ErrNotExist):
		return "", err
	}
	return aside, os.Rename(dest, aside)
}

// fileMode makes shell scripts executable; everything else is a regular file.
func fileMode(p string) os.FileMode {
	if filepath.Ext(p) == ".sh" {
		return ioutil.ExecFile
	}
	return ioutil.File
}
