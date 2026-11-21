package artifact

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTarGz_Install(t *testing.T) {
	const content = "#!/bin/sh\necho antigravity\n"

	// the exec packed WITHOUT the exec bit: Install forces it regardless
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "antigravity", Mode: 0o644, Size: int64(len(content))}))
	_, err := tw.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	d := NewTarGz("antigravity")
	dir := t.TempDir()
	require.NoError(t, d.Install(t.Context(), dir, bytes.NewReader(buf.Bytes())))

	binPath := filepath.Join(dir, "antigravity")
	assert.Equal(t, []byte(content), readFile(t, binPath))
	assert.NotZero(t, stat(t, binPath).Mode().Perm()&0o100, "want the exec bit forced")

	t.Run("bad gzip", func(t *testing.T) {
		err := d.Install(t.Context(), t.TempDir(), bytes.NewReader([]byte("not a tar.gz")))
		assert.ErrorContains(t, err, "extract tar.gz")
	})
}

func TestNewTarGz(t *testing.T) {
	d := NewTarGz("antigravity")
	assert.Equal(t, "antigravity", d.RelBin())
}
