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
		setup      func(t *testing.T, dir string) string // lays the pre-state, returns the path to link
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

			assert.True(t, IsSymlinkTo(path, tt.target), "the symlink is laid at the path")

			if tt.createdDir != "" {
				created, err := os.Stat(filepath.Join(dir, tt.createdDir))
				require.NoError(t, err)
				assert.Equal(t, Dir, created.Mode().Perm())
			}
		})
	}
}

func TestSafeWriteFile(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, dir string) string // lays the pre-state, returns the path to write
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

			err := SafeWriteFile(path, []byte(tt.body))
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
			assert.Equal(t, File, file.Mode().Perm())

			if tt.createdDir != "" {
				created, err := os.Stat(filepath.Join(dir, tt.createdDir))
				require.NoError(t, err)
				assert.Equal(t, Dir, created.Mode().Perm())
			}
		})
	}
}
