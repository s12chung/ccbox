package tarutil

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToTar(t *testing.T) {
	src := fstest.MapFS{
		"a.txt":     {Data: []byte("A")},
		"sub/b.txt": {Data: []byte("B")}, // nested → exercises dir skipping
	}

	r, err := ToTar(src)
	require.NoError(t, err)

	got := map[string]string{}
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		body, err := io.ReadAll(tr)
		require.NoError(t, err)
		got[hdr.Name] = string(body)
	}
	assert.Equal(t, map[string]string{"a.txt": "A", "sub/b.txt": "B"}, got)
}

// tarEntry is one fixture entry: a regular file (body), a symlink (link), or
// whatever type overrides — e.g. tar.TypeDir.
type tarEntry struct {
	name string
	mode os.FileMode
	body string
	link string
	typ  byte
}

func packTar(t *testing.T, entries []tarEntry) []byte {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Mode: int64(e.mode.Perm()), Linkname: e.link}
		switch {
		case e.typ != 0:
			hdr.Typeflag = e.typ
		case e.link != "":
			hdr.Typeflag = tar.TypeSymlink
		default:
			hdr.Typeflag = tar.TypeReg
			hdr.Size = int64(len(e.body))
		}
		require.NoError(t, tw.WriteHeader(hdr))
		if hdr.Typeflag == tar.TypeReg {
			_, err := tw.Write([]byte(e.body))
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
	return buf.Bytes()
}

func TestExtract(t *testing.T) {
	entries := []tarEntry{
		{name: "app/exec", mode: 0o644, body: "exec"},
		{name: "app/secret", mode: 0o600, body: "key"},
		{name: "app/resources", typ: tar.TypeDir, mode: 0o755},
		{name: "app/resources/app.ico", mode: 0o644, body: "ico"},
		{name: "app/bin/agy", link: "../exec"},
	}

	dir := t.TempDir()
	require.NoError(t, Extract(dir, bytes.NewReader(packTar(t, entries))))

	body, err := os.ReadFile(filepath.Join(dir, "app", "exec")) //nolint:gosec // test fixture path
	require.NoError(t, err)
	assert.Equal(t, "exec", string(body))
	// the file's packed perms survive — the extraction chmods past the umask
	assert.Equal(t, 0o600, int(stat(t, filepath.Join(dir, "app", "secret")).Mode().Perm()))
	assert.True(t, stat(t, filepath.Join(dir, "app", "resources")).IsDir(), "dir entry placed")
	stat(t, filepath.Join(dir, "app", "resources", "app.ico"))

	link, err := os.Readlink(filepath.Join(dir, "app", "bin", "agy"))
	require.NoError(t, err)
	assert.Equal(t, "../exec", link)

	t.Run("bad tar", func(t *testing.T) {
		err := Extract(t.TempDir(), bytes.NewReader(bytes.Repeat([]byte("x"), 512)))
		assert.ErrorContains(t, err, "invalid tar header")
	})
}

func TestExtract_Escape(t *testing.T) {
	tests := []struct {
		name    string
		entries []tarEntry
		wantErr string
	}{
		{
			name:    "dot-dot entry",
			entries: []tarEntry{{name: "../evil", body: "x"}},
			wantErr: `entry "../evil" escapes`,
		},
		{
			name:    "absolute entry",
			entries: []tarEntry{{name: "/evil", body: "x"}},
			wantErr: `entry "/evil" escapes`,
		},
		{
			name:    "absolute symlink target",
			entries: []tarEntry{{name: "agy", link: "/evil"}},
			wantErr: `symlink "agy" -> "/evil" escapes`,
		},
		{
			name:    "hardlink entry",
			entries: []tarEntry{{name: "evil", typ: tar.TypeLink}},
			wantErr: `unsupported type`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			err := Extract(dir, bytes.NewReader(packTar(t, tt.entries)))
			require.ErrorContains(t, err, tt.wantErr)

			placed, err := os.ReadDir(dir)
			require.NoError(t, err)
			assert.Empty(t, placed, "nothing placed")
		})
	}
}

func stat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info
}
