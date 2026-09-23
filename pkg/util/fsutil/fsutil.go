// Package fsutil provides fs.FS utilities.
package fsutil

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
)

// errMismatch aborts Matches' walk at the first difference
var errMismatch = errors.New("fsutil: trees differ")

// Matches reports whether dir's tree is exactly fsys's: the same dirs, the same
// file set, and byte-identical contents.
func Matches(fsys fs.FS, dir string) (bool, error) {
	dirs, files, err := tree(fsys)
	if err != nil {
		return false, err
	}

	dfs := os.DirFS(dir)
	err = fs.WalkDir(dfs, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != "." && !dirs[p] {
				return errMismatch
			}
			delete(dirs, p)
			return nil
		}
		body, ok := files[p]
		if !ok {
			return errMismatch
		}
		got, err := fs.ReadFile(dfs, p)
		if err != nil {
			return err
		}
		if !bytes.Equal(got, body) {
			return errMismatch
		}
		delete(files, p)
		return nil
	})
	// leftover entries were never seen in dir: deletions aren't walked
	matched := err == nil && len(dirs) == 0 && len(files) == 0
	// errMismatch is a false, not an error; normalize only after deriving matched
	if errors.Is(err, errMismatch) {
		err = nil
	}
	return matched, err
}

// tree maps fsys's dirs and file bodies by path
func tree(fsys fs.FS) (map[string]bool, map[string][]byte, error) {
	dirs := map[string]bool{}
	files := map[string][]byte{}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != "." {
				dirs[p] = true
			}
			return nil
		}
		body, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		files[p] = body
		return nil
	})
	return dirs, files, err
}
