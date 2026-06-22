// Package embedfs turns an embedded file tree into a tar stream.
package embedfs

import (
	"archive/tar"
	"bytes"
	"io"
	"io/fs"
)

// ToTar packs every file in src into an in-memory tar, skipping directories and
// giving each file mode 0644. It buffers, so it's for small trees (e.g. embeds).
func ToTar(src fs.FS) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	err := fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := fs.ReadFile(src, p)
		if err != nil {
			return err
		}
		if err := tw.WriteHeader(&tar.Header{Name: p, Mode: 0o644, Size: int64(len(body))}); err != nil {
			return err
		}
		_, err = tw.Write(body)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &buf, tw.Close()
}
