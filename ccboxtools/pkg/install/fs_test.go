package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSafeMkdir(t *testing.T) {
	p := filepath.Join(t.TempDir(), "dir")
	require.NoError(t, os.MkdirAll(p, dirMode))
	require.NoError(t, os.WriteFile(filepath.Join(p, "stale"), nil, dirMode))

	require.NoError(t, safeMkdir(p)) // recreated over the existing dir

	entries, err := os.ReadDir(p)
	require.NoError(t, err)
	assert.Empty(t, entries) // stale content wiped
}

func TestSafeMv(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	require.NoError(t, os.MkdirAll(src, dirMode))
	dest := filepath.Join(root, "dest")
	require.NoError(t, os.MkdirAll(dest, dirMode))
	require.NoError(t, os.WriteFile(filepath.Join(dest, "stale"), nil, dirMode))

	require.NoError(t, safeMv(src, dest))

	require.DirExists(t, dest)
	entries, err := os.ReadDir(dest)
	require.NoError(t, err)
	assert.Empty(t, entries) // dest's old content replaced by src
}

func TestReplaceSymlink(t *testing.T) {
	link := filepath.Join(t.TempDir(), "link")

	require.NoError(t, replaceSymlink("a", link))
	assert.Equal(t, "a", readLink(t, link))

	require.NoError(t, replaceSymlink("b", link))
	assert.Equal(t, "b", readLink(t, link))
	assert.NoFileExists(t, link+".tmp")
}

func TestPrune(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"1.0.0", "2.0.0", "current", ".tmp-3.0.0"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, name), dirMode))
	}

	require.NoError(t, prune(dir, "2.0.0"))

	// everything but the active version and current is gone, .tmp leftovers included
	assert.Equal(t, []string{"2.0.0", "current"}, entryNames(t, dir))
}
