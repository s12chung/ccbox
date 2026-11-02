package dock

import (
	"archive/tar"
	"bytes"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/docker/docker/pkg/stdcopy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChownMnt(t *testing.T) {
	assert.Equal(t, "/mnt/ccbox-vol", chownMnt(OwnedVolume{Name: "ccbox-vol"}))
}

func TestChownBind(t *testing.T) {
	assert.Equal(t, "ccbox-vol:/mnt/ccbox-vol", chownBind(OwnedVolume{Name: "ccbox-vol"}))
}

func TestDemuxReader_Read(t *testing.T) {
	big := strings.Repeat("x", 1200)

	var framed bytes.Buffer
	_, err := stdcopy.NewStdWriter(&framed, stdcopy.Stdout).Write([]byte("one\ntwo\n"))
	require.NoError(t, err)
	_, err = stdcopy.NewStdWriter(&framed, stdcopy.Stderr).Write([]byte("three\n"))
	require.NoError(t, err)
	_, err = stdcopy.NewStdWriter(&framed, stdcopy.Stdout).Write([]byte(big))
	require.NoError(t, err)

	// byte-wise reads force a frame's bytes to survive across reads
	got, err := io.ReadAll(iotest.OneByteReader(&demuxReader{r: io.NopCloser(&framed)}))
	require.NoError(t, err)
	assert.Equal(t, "one\ntwo\nthree\n"+big, string(got))
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
