package tarutil

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tarEntries reads the plain tar from r into name → body, failing on duplicate
// or directory entries
func tarEntries(t *testing.T, r io.Reader) map[string]string {
	t.Helper()

	got := map[string]string{}
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return got
		}
		require.NoError(t, err)
		_, dup := got[hdr.Name]
		assert.Falsef(t, dup, "duplicate tar entry: %q", hdr.Name)
		assert.Falsef(t, strings.HasSuffix(hdr.Name, "/"), "dir entry leaked into tar: %q", hdr.Name)
		body, err := io.ReadAll(tr)
		require.NoError(t, err)
		got[hdr.Name] = string(body)
	}
}

// tarBytes packs bodies into the tar headers, gzip-wrapped when gz is set
func tarBytes(t *testing.T, gz bool, entries []tar.Header, bodies map[string][]byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	var w io.Writer = &buf
	if gz {
		w = gzip.NewWriter(&buf)
	}
	tw := tar.NewWriter(w)
	for _, hdr := range entries {
		require.NoError(t, tw.WriteHeader(&hdr))
		_, err := tw.Write(bodies[hdr.Name])
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	if wc, ok := w.(io.Closer); ok {
		require.NoError(t, wc.Close())
	}
	return buf.Bytes()
}

func TestToTar(t *testing.T) {
	src := fstest.MapFS{
		"a.txt":     {Data: []byte("A")},
		"sub/b.txt": {Data: []byte("B")}, // nested → exercises dir skipping
	}

	r, err := ToTar(src, nil, false)
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"a.txt": "A", "sub/b.txt": "B"}, tarEntries(t, r))
}

func TestToTarOverrides(t *testing.T) {
	src := fstest.MapFS{
		"a.txt": {Data: []byte("A")},
	}
	overrides := map[string][]byte{
		"a.txt": []byte("OVERRIDDEN"), // replaces src
		"c.txt": []byte("C"),          // adds
	}

	r, err := ToTar(src, overrides, false)
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"a.txt": "OVERRIDDEN", "c.txt": "C"}, tarEntries(t, r))
}

func TestToTar_Gzip(t *testing.T) {
	src := fstest.MapFS{"a.txt": {Data: []byte("A")}}

	r, err := ToTar(src, nil, true)
	require.NoError(t, err)

	gz, err := gzip.NewReader(r)
	require.NoError(t, err)
	defer func() { require.NoError(t, gz.Close()) }()

	assert.Equal(t, map[string]string{"a.txt": "A"}, tarEntries(t, gz))
}

func TestReadTar(t *testing.T) {
	entries := []tar.Header{
		{Name: "a.txt", Size: 1},
		{Name: "dir/", Typeflag: tar.TypeDir}, // skipped: not a regular file
		{Name: "sub/b.txt", Size: 1},
	}
	bodies := map[string][]byte{"a.txt": []byte("A"), "sub/b.txt": []byte("B")}
	want := map[string][]byte{"a.txt": []byte("A"), "sub/b.txt": []byte("B")}

	t.Run("gzip", func(t *testing.T) {
		got, err := ReadTar(bytes.NewReader(tarBytes(t, true, entries, bodies)))
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("plain", func(t *testing.T) {
		got, err := ReadTar(bytes.NewReader(tarBytes(t, false, entries, bodies)))
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("garbage", func(t *testing.T) {
		_, err := ReadTar(bytes.NewReader([]byte("not a tar at all")))

		require.Error(t, err)
	})
}
