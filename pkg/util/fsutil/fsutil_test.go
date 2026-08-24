package fsutil

import (
	"archive/tar"
	"io"
	"strings"
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

	r, err := ToTar(src, nil)
	require.NoError(t, err)

	got := map[string]string{}
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		assert.Falsef(t, strings.HasSuffix(hdr.Name, "/"), "dir entry leaked into tar: %q", hdr.Name)
		body, err := io.ReadAll(tr)
		require.NoError(t, err)
		got[hdr.Name] = string(body)
	}

	assert.Equal(t, map[string]string{"a.txt": "A", "sub/b.txt": "B"}, got)
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
		_, dup := got[hdr.Name]
		assert.Falsef(t, dup, "duplicate tar entry: %q", hdr.Name)
		got[hdr.Name] = string(body)
	}

	assert.Equal(t, map[string]string{"a.txt": "OVERRIDDEN", "c.txt": "C"}, got)
}
