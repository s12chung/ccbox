// Package seed seeds config content onto host paths: whole trees, backing up any
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

	"github.com/s12chung/ccbox/pkg/util/errs"
	"github.com/s12chung/ccbox/pkg/util/osutil"
)

// ErrExists reports that File's path already exists: an existing file is never
// touched, so there is nothing to seed.
var ErrExists = errors.New("seed: file already exists")

// File seeds path with body when absent, creating its parent dir. An existing file
// is never touched: the returned error wraps ErrExists, carrying the path.
func File(path string, body []byte) error {
	switch _, err := os.Stat(path); {
	case err == nil:
		return fmt.Errorf("%s: %w", path, ErrExists)
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), osutil.Dir); err != nil {
		return err
	}
	return os.WriteFile(path, body, osutil.File)
}

// SafeFile is File with ErrExists swallowed: an existing file — a mid-race
// appearance included — is a no-op, not an error; it is never touched.
func SafeFile(path string, body []byte) error { return errs.Swallow(File(path, body), ErrExists) }

// ErrNoChanges reports that FS made no changes: every destination already
// matched its source, so nothing was written or backed up.
var ErrNoChanges = errors.New("seed: all files identical")

// FS seeds fsys's tree onto destDir (creating it): each differing dest file is
// first infixed to <base>.old<ext>; the infixed paths return, ErrNoChanges when
// none differ.
func FS(fsys fs.FS, destDir string) ([]string, error) {
	infixed, changed, err := sync(fsys, destDir, "old")
	if err == nil && !changed {
		return infixed, ErrNoChanges
	}
	return infixed, err
}

// IfNotExist is FS gated to a missing destDir: a first-run seed, an existing
// dir is never touched.
func IfNotExist(fsys fs.FS, destDir string) error {
	switch _, err := os.Stat(destDir); {
	case err == nil:
		return nil
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	_, err := FS(fsys, destDir)
	return errs.Swallow(err, ErrNoChanges)
}

// Merge moves srcDir's changes into dstDir (creating it): each differing dst
// file is first infixed to <base>.<infix><ext>; the infixed paths return.
func Merge(srcDir, dstDir, infix string) ([]string, error) {
	switch _, err := os.Stat(dstDir); {
	case errors.Is(err, fs.ErrNotExist):
		if err := os.MkdirAll(filepath.Dir(dstDir), osutil.Dir); err != nil {
			return nil, err
		}
		return nil, os.Rename(srcDir, dstDir)
	case err != nil:
		return nil, err
	}
	infixed, _, err := sync(os.DirFS(srcDir), dstDir, infix)
	return infixed, err
}

// sync is FS's walk: a differing dest file is infixed before it's overwritten,
// empty dirs are created as-is; the second return reports that any file was written.
func sync(fsys fs.FS, destDir, infix string) ([]string, bool, error) {
	var infixed []string
	var changed bool
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(destDir, p), osutil.Dir)
		}
		body, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}

		dest := filepath.Join(destDir, p)
		if err := os.MkdirAll(filepath.Dir(dest), osutil.Dir); err != nil {
			return err
		}
		if existing, err := os.ReadFile(dest); err == nil { // #nosec G304 -- dest is the synced tree's own path
			if bytes.Equal(existing, body) {
				return nil
			}
			path, err := infixedDest(dest, infix)
			if err != nil {
				return err
			}
			infixed = append(infixed, path)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		changed = true
		return os.WriteFile(dest, body, fileMode(dest))
	})
	return infixed, changed, err
}

// infixedDest moves dest to <base>.<infix><ext>, refusing to clobber a pre-existing
// infixed file
func infixedDest(dest, infix string) (string, error) {
	ext := filepath.Ext(dest)
	path := strings.TrimSuffix(dest, ext) + "." + infix + ext
	switch _, err := os.Stat(path); {
	case err == nil:
		return "", fmt.Errorf("seed: infixed file already exists, refusing to overwrite: %s", path)
	case !errors.Is(err, fs.ErrNotExist):
		return "", err
	}
	return path, os.Rename(dest, path)
}

// fileMode makes shell scripts executable; everything else is a regular file.
func fileMode(p string) os.FileMode {
	if filepath.Ext(p) == ".sh" {
		return osutil.ExecFile
	}
	return osutil.File
}
