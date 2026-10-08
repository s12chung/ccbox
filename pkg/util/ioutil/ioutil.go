// Package ioutil contains utils for io
package ioutil

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/s12chung/ccbox/pkg/util/errs"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// Modes for the files and directories ccbox writes.
const (
	Dir      os.FileMode = 0o755 // directories
	File     os.FileMode = 0o644 // regular files
	ExecFile os.FileMode = 0o755 // executable files (e.g. shell scripts)
)

// Missing returns whether path does not exist
func Missing(path string) bool {
	_, err := os.Stat(path)
	return errors.Is(err, fs.ErrNotExist)
}

// Present returns whether path exists
func Present(path string) bool { return !Missing(path) }

// SafeWriteFile writes body at path with File perms, creating its parent dir when missing
func SafeWriteFile(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), Dir); err != nil {
		return err
	}
	return os.WriteFile(path, body, File)
}

// AtomicWriteFile writes body at path with File perms via temp file + rename, creating
// its parent dir when missing. A concurrent reader sees either the old or the new file,
// never a partial or missing one.
func AtomicWriteFile(path string, body []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, Dir); err != nil {
		return err
	}

	temp, err := os.CreateTemp(dir, filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer log.Defer("temp file close", temp.Close)
	// a successful rename already moved the temp away: a missing temp is already clean
	defer log.Defer("temp file removal", func() error {
		return errs.Swallow(os.Remove(temp.Name()), fs.ErrNotExist)
	})

	if _, err := temp.Write(body); err != nil {
		return err
	}
	if err := temp.Chmod(File); err != nil { // CreateTemp makes it 0600 — unreadable to other uids
		return err
	}
	return os.Rename(temp.Name(), path)
}

// ClearDir removes dir's contents, keeping the dir itself. A missing dir is already
// clean.
func ClearDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return errs.Swallow(err, fs.ErrNotExist)
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// SafeSymlink symlinks path to target, creating its parent dir when missing. The target
// may be missing: a symlink is never followed to create it.
func SafeSymlink(path, target string) error {
	if err := os.MkdirAll(filepath.Dir(path), Dir); err != nil {
		return err
	}
	return os.Symlink(target, path)
}

// IsSymlinkTo reports whether path is a symlink to target
func IsSymlinkTo(path, target string) bool {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return false
	}
	link, err := os.Readlink(path)
	return err == nil && link == target
}

// DirsPresent returns the entries of dirs that exist as directories under src.
func DirsPresent(src string, dirs []string) []string {
	var out []string
	for _, d := range dirs {
		if info, err := os.Stat(filepath.Join(src, d)); err == nil && info.IsDir() {
			out = append(out, d)
		}
	}
	return out
}

// ExpandHome resolves path's leading ~/ to home.
func ExpandHome(path, home string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
}

// BlockedCloser wraps f with a no-op Close: for files that must outlive whoever
// streams through them (e.g. os.Stdout — closing it frees fd 1 for the next open).
func BlockedCloser(f *os.File) io.WriteCloser {
	return blockedCloser{f}
}

type blockedCloser struct{ *os.File }

func (blockedCloser) Close() error { return nil }
