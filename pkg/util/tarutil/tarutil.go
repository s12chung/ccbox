// Package tarutil contains utils for tar archives
package tarutil

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"io/fs"

	"github.com/s12chung/ccbox/pkg/util/log"
)

// ToTar packs every file in src into an in-memory tar, skipping directories and
// giving each file mode 0644. It buffers, so it's for small trees (e.g. embeds).
// overrides (nil ok) map a path to its bytes, replacing that file in src or adding
// it. gz wraps the tar in gzip (a .tar.gz).
func ToTar(src fs.FS, overrides map[string][]byte, gz bool) (io.Reader, error) {
	var buf bytes.Buffer
	var w io.WriteCloser = nopWriteCloser{&buf}
	if gz {
		w = gzip.NewWriter(&buf)
	}
	tw := tar.NewWriter(w)
	// defer order matters: the tar closes into the gzip before the gzip closes
	// into the buffer
	defer log.Defer("writer close", w.Close)
	defer log.Defer("tar writer close", tw.Close)

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

// nopWriteCloser passes the tar through unwrapped (gzip.Writer can Close(), bytes.Buffer can't)
type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

func writeTar(tw *tar.Writer, name string, body []byte) error {
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}); err != nil {
		return err
	}
	_, err := tw.Write(body)
	return err
}

// gzipMagic is the gzip header's leading bytes, used to sniff wrapped tars
var gzipMagic = []byte{0x1f, 0x8b}

// ReadTar reads the tar from r — gzip-wrapped or not, sniffed by sniffGzip —
// into name → file body, regular files only.
func ReadTar(r io.Reader) (map[string][]byte, error) {
	rc, err := sniffGzip(r)
	if err != nil {
		return nil, err
	}
	defer log.Defer("tar reader close", rc.Close)

	packed := map[string][]byte{}
	tr := tar.NewReader(rc)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return packed, nil
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		packed[header.Name] = body
	}
}

// sniffGzip peeks the gzip magic off r, unwrapping the gzip reader when wrapped
func sniffGzip(r io.Reader) (io.ReadCloser, error) {
	br := bufio.NewReader(r)
	magic, peekErr := br.Peek(len(gzipMagic))
	if peekErr == nil && bytes.Equal(magic, gzipMagic) {
		return gzip.NewReader(br)
	}
	return nopReadCloser{br}, nil
}

// nopReadCloser passes the tar through unwrapped (gzip can Close(), io.Reader can't)
type nopReadCloser struct{ io.Reader }

func (nopReadCloser) Close() error { return nil }
