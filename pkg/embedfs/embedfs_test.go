package embedfs

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
		assert.Falsef(t, strings.HasSuffix(hdr.Name, "/"), "dir entry leaked into tar: %q", hdr.Name)
		body, err := io.ReadAll(tr)
		require.NoError(t, err)
		got[hdr.Name] = string(body)
	}

	assert.Equal(t, map[string]string{"a.txt": "A", "sub/b.txt": "B"}, got)
}
