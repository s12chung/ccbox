package projectcfg

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/deepcopy"
	"github.com/s12chung/ccbox/pkg/perm"
)

// mkDefaultDirs creates the tmpfsDefaults under dir so they get prepended.
func mkDefaultDirs(t *testing.T, dir string) {
	t.Helper()
	for _, d := range tmpfsDefaults {
		require.NoError(t, os.Mkdir(filepath.Join(dir, d), perm.Dir))
	}
}

func TestLoadParses(t *testing.T) {
	dir := t.TempDir()
	mkDefaultDirs(t, dir)
	body := "tmpfs:\n  - dist\n  - build\nenv:\n  FOO: bar\n" +
		"allowlist:\n  - ccbox-defaults\n  - example.com\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte(body), perm.File))

	c, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{".idea", ".vscode", "dist", "build"}, c.Tmpfs) // present defaults prepended
	assert.Equal(t, map[string]string{"FOO": "bar"}, c.Env)
	assert.Equal(t, append(append([]string{}, allowDefaults...), "example.com"), c.Allowlist) // token expanded
}

func TestLoadMissingGetsDefaults(t *testing.T) {
	dir := t.TempDir()
	mkDefaultDirs(t, dir)
	c, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, tmpfsDefaults, c.Tmpfs)     // present always-on masks
	assert.Equal(t, allowDefaults, c.Allowlist) // empty allowlist → the built-ins
}

func TestDefaultedResolves(t *testing.T) {
	dir := t.TempDir()
	mkDefaultDirs(t, dir)
	c := Defaulted(dir, Config{Tmpfs: []string{"dist"}, Allowlist: []string{"example.com"}})
	assert.Equal(t, []string{".idea", ".vscode", "dist"}, c.Tmpfs)
	assert.Equal(t, []string{"example.com"}, c.Allowlist) // no token → no defaults pulled in
}

func TestDefaultedSkipsAbsentTmpfsDefaults(t *testing.T) {
	dir := t.TempDir() // no .idea/.vscode on disk
	c := Defaulted(dir, Config{Tmpfs: []string{"dist"}})
	assert.Equal(t, []string{"dist"}, c.Tmpfs) // absent defaults not prepended
}

func TestDefaultedVolumesPresentPrepended(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "node_modules"), perm.Dir))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "vendor", "bundle"), perm.Dir)) // nested default

	c := Defaulted(dir, Config{Volumes: []string{"target"}})
	// only present defaults prepended (.venv absent), user entry kept after
	assert.Equal(t, []string{"node_modules", "vendor/bundle", "target"}, c.Volumes)
}

func TestDefaultedSkipsAbsentVolumeDefaults(t *testing.T) {
	dir := t.TempDir() // no node_modules/.venv/vendor on disk
	c := Defaulted(dir, Config{})
	assert.Empty(t, c.Volumes) // absent defaults not prepended
}

func TestVolumeCleanupDirs(t *testing.T) {
	// all defaults regardless of presence, plus explicit config volumes, deduped
	c := Config{Volumes: []string{"node_modules", "target"}}
	assert.Equal(t, []string{"node_modules", ".venv", "vendor/bundle", "target"}, c.VolumeCleanupDirs())
}

func TestInitWritesLoadableDefault(t *testing.T) {
	dir := t.TempDir()
	path, err := Init(dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, fileName), path)

	c, err := Load(dir) // the written scaffold must resolve to the no-file behavior
	require.NoError(t, err)
	assert.Equal(t, Defaulted(dir, Config{}).Tmpfs, c.Tmpfs)
	assert.Equal(t, Defaulted(dir, Config{}).Allowlist, c.Allowlist)
	assert.Empty(t, c.Env)
}

func TestInitRefusesExisting(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte("tmpfs: []\n"), perm.File))

	_, err := Init(dir)
	assert.ErrorContains(t, err, "already exists")
}

func TestDefaultedAllowlistNilVsEmpty(t *testing.T) {
	dir := t.TempDir()
	assert.Equal(t, allowDefaults, Defaulted(dir, Config{Allowlist: nil}).Allowlist) // unset → built-ins
	assert.Empty(t, Defaulted(dir, Config{Allowlist: []string{}}).Allowlist)         // explicit [] → nothing
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
	mkDefaultDirs(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte(""), perm.File))

	c, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, allowDefaults, c.Allowlist) // an empty file is unset, not []
	assert.Equal(t, tmpfsDefaults, c.Tmpfs)
}

func TestLoadMergesLocalOverride(t *testing.T) {
	dir := t.TempDir()
	mkDefaultDirs(t, dir)
	base := "tmpfs:\n  - dist\nenv:\n  FOO: base\n  BAR: base\nallowlist:\n  - ccbox-defaults\n"
	local := "tmpfs:\n  - build\nenv:\n  FOO: local\nallowlist:\n  - example.com\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte(base), perm.File))
	require.NoError(t, os.WriteFile(filepath.Join(dir, localFileName), []byte(local), perm.File))

	c, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{".idea", ".vscode", "dist", "build"}, c.Tmpfs)                   // lists append, base first
	assert.Equal(t, map[string]string{"FOO": "local", "BAR": "base"}, c.Env)                  // env overlays, local wins
	assert.Equal(t, append(append([]string{}, allowDefaults...), "example.com"), c.Allowlist) // merged, then token expanded
}

func TestLoadLocalOnly(t *testing.T) {
	dir := t.TempDir() // no base .ccbox.yaml
	require.NoError(t, os.WriteFile(filepath.Join(dir, localFileName),
		[]byte("allowlist:\n  - example.com\n"), perm.File))

	c, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"example.com"}, c.Allowlist) // no base, no token → just the override
}

func TestLoadInvalidLocalErrors(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, localFileName), []byte("tmpfs: ["), perm.File))

	_, err := Load(dir)
	assert.Error(t, err)
}

func TestLoadInvalidErrors(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte("tmpfs: ["), perm.File))

	_, err := Load(dir)
	assert.Error(t, err)
}

func TestMergeIsPure(t *testing.T) {
	src := Config{
		Tmpfs:     []string{"dist"},
		Volumes:   []string{"target"},
		Env:       map[string]string{"FOO": "base", "BAR": "base"},
		Allowlist: []string{"ccbox-defaults"},
	}
	other := Config{
		Tmpfs:     []string{"build"},
		Volumes:   []string{"cache"},
		Env:       map[string]string{"FOO": "local", "BAZ": "local"},
		Allowlist: []string{"example.com"},
	}
	before := deepcopy.Of(src)

	got := src.merge(other)

	// merge must not mutate its receiver — every src field still equals its pre-merge snapshot
	srvVal, beforeVal, gotVal := reflect.ValueOf(src), reflect.ValueOf(before), reflect.ValueOf(got)
	for i := 0; i < srvVal.NumField(); i++ {
		field := srvVal.Type().Field(i).Name
		assert.Equal(t, beforeVal.Field(i).Interface(), srvVal.Field(i).Interface(), "merge mutated src.%s", field)
		assert.NotEqual(t, beforeVal.Field(i).Interface(), gotVal.Field(i).Interface(), "before = got on field %s, maybe missed a new field?", field)
	}

	// and the returned Config is the actual layering
	assert.Equal(t, []string{"dist", "build"}, got.Tmpfs)
	assert.Equal(t, []string{"target", "cache"}, got.Volumes)
	assert.Equal(t, map[string]string{"FOO": "local", "BAR": "base", "BAZ": "local"}, got.Env)
	assert.Equal(t, []string{"ccbox-defaults", "example.com"}, got.Allowlist)
}
