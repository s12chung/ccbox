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
	body := "tmpfs:\n  - dist\n  - build\nenv:\n  FOO: bar\n" +
		"allowlist:\n  defaults: false\n  domains:\n    - example.com\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte(body), perm.File))

	c, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"dist", "build"}, c.Tmpfs)
	assert.Equal(t, map[string]string{"FOO": "bar"}, c.Env)
	assert.False(t, c.Allowlist.DefaultsEnabled())
	assert.Equal(t, []string{"example.com"}, c.Allowlist.Domains)
}

func TestDefaultsEnabledWhenUnset(t *testing.T) {
	c, err := Load(t.TempDir()) // no file → Defaults nil
	require.NoError(t, err)
	assert.True(t, c.Allowlist.DefaultsEnabled())
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
