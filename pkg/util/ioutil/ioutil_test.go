package ioutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file")
	require.NoError(t, os.WriteFile(path, nil, File))

	assert.False(t, Missing(path))
	assert.False(t, Missing(dir), "dirs count as existing")
	assert.True(t, Missing(filepath.Join(dir, "missing")))
	assert.True(t, Missing(filepath.Join(dir, "no-dir", "missing")))
}

func TestDirs_Present(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "present/nested"), Dir))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "afile"), nil, File))

	tests := []struct {
		name string
		dirs []string
		want []string
	}{
		{"all present, in order", []string{"present", "present/nested"}, []string{"present", "present/nested"}},
		{"absent drops out", []string{"missing", "present"}, []string{"present"}},
		{"a file doesn't count", []string{"afile", "present"}, []string{"present"}},
		{"nothing present", []string{"missing", "also-missing"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, DirsPresent(dir, tt.dirs))
		})
	}
}

func TestIsSymlinkTo(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(target, link))
	file := filepath.Join(dir, "file")
	require.NoError(t, os.WriteFile(file, nil, File))

	assert.True(t, IsSymlinkTo(link, target))
	assert.False(t, IsSymlinkTo(link, filepath.Join(dir, "elsewhere")), "a symlink elsewhere")
	assert.False(t, IsSymlinkTo(file, target), "a regular file")
	assert.False(t, IsSymlinkTo(filepath.Join(dir, "missing"), target), "a missing path")
}

func TestSafeSymlink(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, dir string) string // writes the pre-state, returns the path to link
		target     string                                // the symlink's target
		err        bool                                  // whether the link errs
		createdDir string                                // non-empty: dir the link creates, asserted with Dir perms
	}{
		{
			name:       "creates the parent dir",
			setup:      func(_ *testing.T, dir string) string { return filepath.Join(dir, "missing/parent/link") },
			target:     "missing/target",
			createdDir: "missing/parent",
		},
		{
			name: "errs when the path exists",
			setup: func(t *testing.T, dir string) string {
				path := filepath.Join(dir, "file")
				require.NoError(t, os.WriteFile(path, nil, File))
				return path
			},
			target: "target",
			err:    true,
		},
		{
			name: "errs when the parent is a file",
			setup: func(t *testing.T, dir string) string {
				parent := filepath.Join(dir, "afile")
				require.NoError(t, os.WriteFile(parent, nil, File))
				return filepath.Join(parent, "link")
			},
			target: "target",
			err:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := tt.setup(t, dir)

			err := SafeSymlink(path, tt.target)
			if tt.err {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			assert.True(t, IsSymlinkTo(path, tt.target), "the symlink is created at the path")

			if tt.createdDir != "" {
				created, err := os.Stat(filepath.Join(dir, tt.createdDir))
				require.NoError(t, err)
				assert.Equal(t, Dir, created.Mode().Perm())
			}
		})
	}
}

func TestSafeWriteFile(t *testing.T)   { testWriteFile(t, SafeWriteFile) }
func TestAtomicWriteFile(t *testing.T) { testWriteFile(t, AtomicWriteFile) }

// testWriteFile runs the writers' shared contract: File perms, parent-dir creation and
// overwrite, no leftovers in the target dir
func testWriteFile(t *testing.T, write func(path string, body []byte) error) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, dir string) string // writes the pre-state, returns the path to write
		body       string
		err        bool   // whether the write errs
		createdDir string // non-empty: dir the write creates, asserted with Dir perms
	}{
		{
			name:       "creates the parent dir",
			setup:      func(_ *testing.T, dir string) string { return filepath.Join(dir, "missing/parent/file") },
			body:       "body",
			createdDir: "missing/parent",
		},
		{
			name: "overwrites an existing file",
			setup: func(t *testing.T, dir string) string {
				path := filepath.Join(dir, "file")
				require.NoError(t, os.WriteFile(path, []byte("old"), File))
				return path
			},
			body: "new",
		},
		{
			name: "errs when the parent is a file",
			setup: func(t *testing.T, dir string) string {
				parent := filepath.Join(dir, "afile")
				require.NoError(t, os.WriteFile(parent, nil, File))
				return filepath.Join(parent, "file")
			},
			body: "body",
			err:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := tt.setup(t, dir)

			err := write(path, []byte(tt.body))
			if tt.err {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			body, err := os.ReadFile(path) // #nosec G304 -- the test's own path
			require.NoError(t, err)
			assert.Equal(t, tt.body, string(body))

			file, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, File, file.Mode().Perm(), "the temp file's 0600 does not leak")

			if tt.createdDir != "" {
				created, err := os.Stat(filepath.Join(dir, tt.createdDir))
				require.NoError(t, err)
				assert.Equal(t, Dir, created.Mode().Perm())
			}

			// no temp file is left behind for a globbing reader to trip on
			entries, err := os.ReadDir(filepath.Dir(path))
			require.NoError(t, err)
			assert.Len(t, entries, 1, "only the written file remains")
		})
	}
}

func TestClearDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dir")

	// a missing dir is already clean
	require.NoError(t, ClearDir(dir))

	require.NoError(t, SafeWriteFile(filepath.Join(dir, "file"), nil))
	require.NoError(t, SafeWriteFile(filepath.Join(dir, "sub/nested"), nil))

	require.NoError(t, ClearDir(dir))

	require.True(t, Present(dir), "the dir itself stays")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestExpandHome(t *testing.T) {
	assert.Equal(t, "/home/me/x", ExpandHome("~/x", "/home/me"))
	assert.Equal(t, "/home/me/x/y", ExpandHome("~/x/y", "/home/me"))
	assert.Equal(t, "/abs/x", ExpandHome("/abs/x", "/home/me"))
	assert.Equal(t, "relative/x", ExpandHome("relative/x", "/home/me"))
	assert.Equal(t, "~x", ExpandHome("~x", "/home/me"), "only the ~/ prefix resolves")
}
