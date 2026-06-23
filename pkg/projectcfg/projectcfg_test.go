package projectcfg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/perm"
)

func TestLoadParses(t *testing.T) {
	dir := t.TempDir()
	body := "tmpfs:\n  - dist\n  - build\nenv:\n  FOO: bar\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte(body), perm.File))

	c, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"dist", "build"}, c.Tmpfs)
	assert.Equal(t, map[string]string{"FOO": "bar"}, c.Env)
}

func TestLoadMissingIsZero(t *testing.T) {
	c, err := Load(t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, Config{}, c)
}

func TestLoadInvalidErrors(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte("tmpfs: ["), perm.File))

	_, err := Load(dir)
	assert.Error(t, err)
}
