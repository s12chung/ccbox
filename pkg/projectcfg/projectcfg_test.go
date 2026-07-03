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
		"allowlist:\n  - ccbox-defaults\n  - example.com\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte(body), perm.File))

	c, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{".idea", ".vscode", "dist", "build"}, c.Tmpfs) // defaults prepended
	assert.Equal(t, map[string]string{"FOO": "bar"}, c.Env)
	assert.Equal(t, append(append([]string{}, allowDefaults...), "example.com"), c.Allowlist) // token expanded
}

func TestLoadMissingGetsDefaults(t *testing.T) {
	c, err := Load(t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, tmpfsDefaults, c.Tmpfs)     // always-on masks
	assert.Equal(t, allowDefaults, c.Allowlist) // empty allowlist → the built-ins
}

func TestDefaultedResolves(t *testing.T) {
	c := Defaulted(Config{Tmpfs: []string{"dist"}, Allowlist: []string{"example.com"}})
	assert.Equal(t, []string{".idea", ".vscode", "dist"}, c.Tmpfs)
	assert.Equal(t, []string{"example.com"}, c.Allowlist) // no token → no defaults pulled in
}

func TestInitWritesLoadableDefault(t *testing.T) {
	dir := t.TempDir()
	path, err := Init(dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, fileName), path)

	c, err := Load(dir) // the written scaffold must resolve to the no-file behavior
	require.NoError(t, err)
	assert.Equal(t, Defaulted(Config{}).Tmpfs, c.Tmpfs)
	assert.Equal(t, Defaulted(Config{}).Allowlist, c.Allowlist)
	assert.Empty(t, c.Env)
}

func TestInitRefusesExisting(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte("tmpfs: []\n"), perm.File))

	_, err := Init(dir)
	assert.ErrorContains(t, err, "already exists")
}

func TestDefaultedAllowlistNilVsEmpty(t *testing.T) {
	assert.Equal(t, allowDefaults, Defaulted(Config{Allowlist: nil}).Allowlist) // unset → built-ins
	assert.Empty(t, Defaulted(Config{Allowlist: []string{}}).Allowlist)         // explicit [] → nothing
}

func TestLoadEmptyAllowlistAllowsNothing(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte("allowlist: []\n"), perm.File))

	c, err := Load(dir)
	require.NoError(t, err)
	assert.Empty(t, c.Allowlist) // an explicit [] is not the built-ins
}

func TestLoadEmptyFileGetsDefaults(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte(""), perm.File))

	c, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, allowDefaults, c.Allowlist) // an empty file is unset, not []
	assert.Equal(t, tmpfsDefaults, c.Tmpfs)
}

func TestLoadInvalidErrors(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte("tmpfs: ["), perm.File))

	_, err := Load(dir)
	assert.Error(t, err)
}
