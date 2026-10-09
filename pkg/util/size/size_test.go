package size

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeSparse creates path holding size bytes without writing them to disk
func writeSparse(t *testing.T, path string, size int64) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, nil, 0o600))
	require.NoError(t, os.Truncate(path, size))
}

func TestDirOver(t *testing.T) {
	t.Run("under", func(t *testing.T) {
		dir := t.TempDir()
		writeSparse(t, filepath.Join(dir, "small"), 10)

		over, err := DirOver(dir, 1<<30)
		require.NoError(t, err)
		assert.False(t, over)
	})

	t.Run("over", func(t *testing.T) {
		dir := t.TempDir()
		writeSparse(t, filepath.Join(dir, "big"), 2<<30)

		over, err := DirOver(dir, 1<<30)
		require.NoError(t, err)
		assert.True(t, over)
	})

	t.Run("symlink not followed", func(t *testing.T) {
		dir := t.TempDir()
		outside := filepath.Join(t.TempDir(), "outside")
		writeSparse(t, outside, 2<<30)
		require.NoError(t, os.Symlink(outside, filepath.Join(dir, "link")))

		over, err := DirOver(dir, 1<<30)
		require.NoError(t, err)
		assert.False(t, over)
	})

	t.Run("missing dir", func(t *testing.T) {
		_, err := DirOver(filepath.Join(t.TempDir(), "missing"), 1<<30)
		require.Error(t, err)
	})
}
