// Package tarutil packs file trees into tars and extracts tars into a target
// dir, entries contained against escapes.
package tarutil

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// ToTar packs every file in src into an in-memory tar, skipping directories and
// giving each file mode 0644. It buffers, so it's for small trees (e.g. embeds).
func ToTar(src fs.FS) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	defer log.Defer("tar writer close", tw.Close)

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
	return &buf, nil
}

// Extract unpacks the tar archive stream r into dir, entry by entry. An entry
// whose name or symlink target escapes dir fails the extraction.
func Extract(dir string, r io.Reader) error {
	reader := tar.NewReader(r)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := extractEntry(dir, header, reader); err != nil {
			return err
		}
	}
}

// extractEntry places one entry into dir: its dirs, regular files, and symlinks.
func extractEntry(dir string, header *tar.Header, body io.Reader) error {
	path, ok := containedPath(dir, header.Name)
	if !ok {
		return fmt.Errorf("tar entry %q escapes %s", header.Name, dir)
	}

	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(path, header.FileInfo().Mode().Perm())
	case tar.TypeReg:
		mode := header.FileInfo().Mode().Perm()
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { // tarballs don't always carry dir entries
			return err
		}
		//nolint:gosec // path is the extraction dir + a contained tar entry name
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
		if err != nil {
			return err
		}
		if _, err := io.Copy(file, body); err != nil { //nolint:gosec // the body's vetting is the caller's concern
			return errors.Join(err, file.Close())
		}
		return errors.Join(file.Chmod(mode), file.Close()) // O_CREATE's perm is umask-masked
	case tar.TypeSymlink:
		// resolve the target the way the OS will, then require it inside dir
		target := header.Linkname
		if !filepath.IsAbs(target) {
			//nolint:gosec // G305: the join is contained below
			target = filepath.Join(dir, filepath.Dir(header.Name), target)
		}
		if !strings.HasPrefix(target, dir+string(filepath.Separator)) {
			return fmt.Errorf("tar symlink %q -> %q escapes %s", header.Name, header.Linkname, dir)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return err
		}
		return os.Symlink(header.Linkname, path)
	default:
		return fmt.Errorf("tar entry %q: unsupported type %q", header.Name, rune(header.Typeflag))
	}
}

// containedPath joins dir with name, ok false when the join escapes dir — an
// entry's name may be absolute or dot-dotted.
func containedPath(dir, name string) (string, bool) {
	if filepath.IsAbs(name) {
		return "", false
	}
	path := filepath.Join(dir, name)
	return path, strings.HasPrefix(path, dir+string(filepath.Separator))
}
