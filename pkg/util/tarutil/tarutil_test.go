package tarutil

import (
	"archive/tar"
	"io"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tarEntries reads the tar from r into name → body, failing on duplicate or
// directory entries
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

func TestToTar(t *testing.T) {
	src := fstest.MapFS{
		"a.txt":     {Data: []byte("A")},
		"sub/b.txt": {Data: []byte("B")}, // nested → exercises dir skipping
	}

	r, err := ToTar(src, nil)
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

	r, err := ToTar(src, overrides)
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"a.txt": "OVERRIDDEN", "c.txt": "C"}, tarEntries(t, r))
}
