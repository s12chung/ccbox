// Package fsutil provides utilities for io/fs
package fsutil

import (
	"archive/tar"
	"bytes"
	"io"
	"io/fs"

	"github.com/s12chung/ccbox/pkg/util/log"
)

// MustSub returns fsys rooted at dir (see fs.Sub), panicking on error. The
// error is unreachable when dir names a compile-time //go:embed pattern root.
func MustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

// RenamedFS is an fs.FS whose files land under other names when laid onto a dest.
type RenamedFS struct {
	FS      fs.FS
	Renames map[string]string // source path -> destination name
}

// RenamedFSes is an ordered set of trees seeded onto one dest dir.
type RenamedFSes struct {
	FSes []RenamedFS
}

// NewRenamedFSes wraps each fsys as a rename-less RenamedFS.
func NewRenamedFSes(fsyses ...fs.FS) RenamedFSes {
	fsList := make([]RenamedFS, len(fsyses))
	for i, fsys := range fsyses {
		fsList[i] = RenamedFS{FS: fsys}
	}
	return RenamedFSes{FSes: fsList}
}

// ToTar packs every file in src into an in-memory tar, skipping directories and
// giving each file mode 0644. It buffers, so it's for small trees (e.g. embeds).
// overrides (nil ok) map a path to its bytes, replacing that file in src or adding it.
func ToTar(src fs.FS, overrides map[string][]byte) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	defer log.Defer("tarWriter close", tw.Close)

	err := fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if _, overridden := overrides[p]; overridden { // written below, from overrides
			return nil
		}
		body, err := fs.ReadFile(src, p)
		if err != nil {
			return err
		}
		return writeTar(tw, p, body)
	})
	if err != nil {
		return nil, err
	}
	for name, body := range overrides {
		if err := writeTar(tw, name, body); err != nil {
			return nil, err
		}
	}
	return &buf, nil
}

func writeTar(tw *tar.Writer, name string, body []byte) error {
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}); err != nil {
		return err
	}
	_, err := tw.Write(body)
	return err
}
