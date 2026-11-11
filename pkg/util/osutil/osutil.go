// Package osutil holds file utilities beyond the standard os package.
package osutil

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
	Dir        os.FileMode = 0o755 // directories
	File       os.FileMode = 0o644 // regular files
	ExecFile   os.FileMode = 0o755 // executable files (e.g. shell scripts)
	SecretFile os.FileMode = 0o600 // secret files, owner-only
)

// Missing returns whether path does not exist
func Missing(path string) bool {
	_, err := os.Stat(path)
	return errors.Is(err, fs.ErrNotExist)
}

// Present returns whether path exists
func Present(path string) bool { return !Missing(path) }

// PickLayer returns the first present of paths, in the order given; "" when none is present
func PickLayer(paths ...string) string {
	for _, path := range paths {
		if Present(path) {
			return path
		}
	}
	return ""
}

// SafeWriteFile writes body at path with File perms, creating its parent dir when missing
func SafeWriteFile(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), Dir); err != nil {
		return err
	}
	return os.WriteFile(path, body, File)
}

// OpenAppend opens path for appending, creating it when missing
func OpenAppend(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, File) // #nosec G304 -- the caller's file
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

// PruneImplied drops each dir implied by a deeper one and duplicate dirs.
// It requires sorted dirs and compacts in place.
func PruneImplied(dirs []string) []string {
	if len(dirs) < 2 {
		return dirs
	}
	out := dirs[:0]
	// possible is the first index a deeper dir could still sit at
	possible := 0
	for i, dir := range dirs {
		// dir+"/" marks a deeper dir's ("/a/" + "deeper"): shift possible up until it's possible again
		for possible < len(dirs) && dirs[possible] < dir+"/" {
			possible++
		}
		if possible < len(dirs) && strings.HasPrefix(dirs[possible], dir+"/") {
			continue // matches possible prefix, skip
		}
		if i > 0 && dir == dirs[i-1] { // sorted: duplicates are adjacent
			continue
		}
		out = append(out, dir)
	}
	return out
}

// BlockedCloser wraps f with a no-op Close: for files that must outlive whoever
// streams through them (e.g. os.Stdout — closing it frees fd 1 for the next open).
func BlockedCloser(f *os.File) io.WriteCloser {
	return blockedCloser{f}
}

type blockedCloser struct{ *os.File }

func (blockedCloser) Close() error { return nil }
