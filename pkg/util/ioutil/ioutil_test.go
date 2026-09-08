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
