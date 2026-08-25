package fsutil

import (
	"errors"
	"io"
	"io/fs"
	"path"
	"time"
)

// fakeDir serves a directory of the merged tree: its listing comes from the
// tree, not from any single underlying fs.FS.
type fakeDir struct {
	fsys *FS
	node *fsnode
	path string
	pos  int
}

func (d *fakeDir) Stat() (fs.FileInfo, error) { return d.fsys.statNode(d.node, d.path) }

func (d *fakeDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: opRead, Path: d.path, Err: errors.New("is a directory")}
}

func (d *fakeDir) Close() error { return nil }

func (d *fakeDir) ReadDir(n int) ([]fs.DirEntry, error) {
	entries := make([]fs.DirEntry, 0, len(d.node.children))
	for _, child := range d.node.children {
		info, err := d.fsys.statNode(&child, path.Join(d.path, child.name))
		if err != nil {
			return nil, &fs.PathError{Op: opReaddir, Path: d.path, Err: err}
		}
		entries = append(entries, namedEntry{name: child.name, info: info})
	}
	remaining := entries[d.pos:]
	if n > 0 && len(remaining) > n {
		remaining = remaining[:n]
	}
	d.pos += len(remaining)
	if n > 0 && len(remaining) == 0 {
		return nil, io.EOF
	}
	return remaining, nil
}

// namedFile serves an opened file under its tree name, which differs from the
// owner's name for renamed nodes.
type namedFile struct {
	fs.File

	name string
}

func (f namedFile) Stat() (fs.FileInfo, error) {
	info, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return fileInfo{FileInfo: info, name: f.name}, nil
}

// fileInfo reports the tree name for content owned elsewhere.
type fileInfo struct {
	fs.FileInfo

	name string
}

func (i fileInfo) Name() string { return i.name }

// dirInfo is a merged-tree directory's FileInfo.
type dirInfo struct{ name string }

func (i dirInfo) Name() string       { return i.name }
func (i dirInfo) Size() int64        { return 0 }
func (i dirInfo) Mode() fs.FileMode  { return fs.ModeDir | 0o555 }
func (i dirInfo) ModTime() time.Time { return time.Time{} }
func (i dirInfo) IsDir() bool        { return true }
func (i dirInfo) Sys() any           { return nil }

// namedEntry reports a tree name over owner-derived info.
type namedEntry struct {
	info fs.FileInfo
	name string
}

func (e namedEntry) Name() string               { return e.name }
func (e namedEntry) IsDir() bool                { return e.info.IsDir() }
func (e namedEntry) Type() fs.FileMode          { return e.info.Mode().Type() }
func (e namedEntry) Info() (fs.FileInfo, error) { return e.info, nil }
