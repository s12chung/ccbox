package installutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readLink(t *testing.T, path string) string {
	t.Helper()
	target, err := os.Readlink(path)
	require.NoError(t, err)
	return target
}

func entryNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestSafeMkdir(t *testing.T) {
	p := filepath.Join(t.TempDir(), "dir")
	require.NoError(t, os.MkdirAll(p, DirMode))
	require.NoError(t, os.WriteFile(filepath.Join(p, "stale"), nil, DirMode))

	require.NoError(t, SafeMkdir(p)) // recreated over the existing dir

	entries, err := os.ReadDir(p)
	require.NoError(t, err)
	assert.Empty(t, entries) // stale content wiped
}

func TestSafeMv(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	require.NoError(t, os.MkdirAll(src, DirMode))
	dest := filepath.Join(root, "dest")
	require.NoError(t, os.MkdirAll(dest, DirMode))
	require.NoError(t, os.WriteFile(filepath.Join(dest, "stale"), nil, DirMode))

	require.NoError(t, SafeMv(src, dest))

	require.DirExists(t, dest)
	entries, err := os.ReadDir(dest)
	require.NoError(t, err)
	assert.Empty(t, entries) // dest's old content replaced by src
}

func TestReplaceSymlink(t *testing.T) {
	link := filepath.Join(t.TempDir(), "link")

	require.NoError(t, ReplaceSymlink("a", link))
	assert.Equal(t, "a", readLink(t, link))

	require.NoError(t, ReplaceSymlink("b", link))
	assert.Equal(t, "b", readLink(t, link))
	assert.NoFileExists(t, link+".tmp")
}

func TestCurrentVersion(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "current")

	assert.Empty(t, CurrentVersion(link)) // unset

	require.NoError(t, os.Symlink(filepath.Join(dir, "1.2.3"), link))
	assert.Equal(t, "1.2.3", CurrentVersion(link))
}

func TestPrune(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"1.0.0", "2.0.0", "current", ".tmp-3.0.0"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, name), DirMode))
	}

	require.NoError(t, Prune(dir, "2.0.0"))

	// everything but the active version and current is gone, .tmp leftovers included
	assert.Equal(t, []string{"2.0.0", "current"}, entryNames(t, dir))
}
