package dock

import (
	"archive/tar"
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChownMnt(t *testing.T) {
	assert.Equal(t, "/mnt/ccbox-vol", chownMnt(OwnedVolume{Name: "ccbox-vol"}))
}

func TestChownBind(t *testing.T) {
	assert.Equal(t, "ccbox-vol:/mnt/ccbox-vol", chownBind(OwnedVolume{Name: "ccbox-vol"}))
}

func TestFilesFromTar(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	dirs := []string{"tree", "tree/nested"}
	for _, dir := range dirs {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: dir, Typeflag: tar.TypeDir}))
	}
	for name, body := range map[string]string{
		"tree/nested/a.txt": "a",
		"tree/b.txt":        "b",
	} {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Size: int64(len(body))}))
		_, err := tw.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())

	files, err := filesFromTar(&buf, "tree/")
	require.NoError(t, err)
	assert.Equal(t, map[string][]byte{
		"nested/a.txt": []byte("a"),
		"b.txt":        []byte("b"),
	}, files, "the files keyed by their paths under the tree, dirs dropped")
}
