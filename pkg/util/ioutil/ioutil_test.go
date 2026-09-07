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
