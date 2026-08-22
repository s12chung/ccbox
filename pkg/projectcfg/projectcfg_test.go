package projectcfg

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/util/deepcopy"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

// mkDefaultDirs creates the tmpfsDefaults under dir so they get prepended.
func mkDefaultDirs(t *testing.T, dir string) {
	t.Helper()
	for _, d := range tmpfsDefaults {
		require.NoError(t, os.Mkdir(filepath.Join(dir, d), ioutil.Dir))
	}
}

func TestLoadParses(t *testing.T) {
	dir := t.TempDir()
	mkDefaultDirs(t, dir)
	body := "cli: codex\ntmpfs:\n  - dist\n  - build\nenv:\n  FOO: bar\n" +
		"allowlist:\n  - ccbox-defaults\n  - example.com\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte(body), ioutil.File))

	c, err := Load(dir, Config{})
	require.NoError(t, err)
	assert.Equal(t, harness.NameCodex, c.CLI)
	assert.Equal(t, []string{".idea", ".vscode", "dist", "build"}, c.Tmpfs) // present defaults prepended
	assert.Equal(t, map[string]string{"FOO": "bar"}, c.Env)
	assert.Equal(t, append(append([]string{}, allowDefaults...), "example.com"), c.Allowlist) // token expanded
}

func TestLoadUnsetGetsDefaults(t *testing.T) {
	// a missing file and an empty file are both "unset" (not []) → the built-in defaults
	for _, tt := range []struct {
		name  string
		write bool
	}{
		{"missing file", false},
		{"empty file", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			mkDefaultDirs(t, dir)
			if tt.write {
				require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte(""), ioutil.File))
			}
			c, err := Load(dir, Config{})
			require.NoError(t, err)
			assert.Equal(t, harness.NameClaude, c.CLI)  // unset cli → claude
			assert.Equal(t, tmpfsDefaults, c.Tmpfs)     // present always-on masks
			assert.Equal(t, allowDefaults, c.Allowlist) // unset allowlist → the built-ins
		})
	}
}

func TestDefaultedResolves(t *testing.T) {
	dir := t.TempDir()
	mkDefaultDirs(t, dir)
	c := Defaulted(dir, Config{Tmpfs: []string{"dist"}, Allowlist: []string{"example.com"}})
	assert.Equal(t, []string{".idea", ".vscode", "dist"}, c.Tmpfs)
	assert.Equal(t, []string{"example.com"}, c.Allowlist) // no token → no defaults pulled in
	assert.Equal(t, new(true), c.HostGitConfig)           // unset → default on
}

func TestDefaultedSkipsAbsentTmpfsDefaults(t *testing.T) {
	dir := t.TempDir() // no .idea/.vscode on disk
	c := Defaulted(dir, Config{Tmpfs: []string{"dist"}})
	assert.Equal(t, []string{"dist"}, c.Tmpfs) // absent defaults not prepended
}

func TestDefaultedVolumesPresentPrepended(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "node_modules"), ioutil.Dir))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "vendor", "bundle"), ioutil.Dir)) // nested default

	c := Defaulted(dir, Config{Volumes: []string{"target"}})
	// only present defaults prepended (.venv absent), user entry kept after
	assert.Equal(t, []string{"node_modules", "vendor/bundle", "target"}, c.Volumes)
}

func TestDefaultedSkipsAbsentVolumeDefaults(t *testing.T) {
	dir := t.TempDir() // no node_modules/.venv/vendor on disk
	c := Defaulted(dir, Config{})
	assert.Empty(t, c.Volumes) // absent defaults not prepended
}

func TestInitWritesLoadableDefault(t *testing.T) {
	dir := t.TempDir()
	path, err := Init(dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, fileName), path)

	c, err := Load(dir, Config{}) // the written scaffold must resolve to the no-file behavior
	require.NoError(t, err)
	assert.Equal(t, Defaulted(dir, Config{}).Tmpfs, c.Tmpfs)
	assert.Equal(t, Defaulted(dir, Config{}).Allowlist, c.Allowlist)
	assert.Empty(t, c.Env)
}

func TestInitRefusesExisting(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte("tmpfs: []\n"), ioutil.File))

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
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte("allowlist: []\n"), ioutil.File))

	c, err := Load(dir, Config{})
	require.NoError(t, err)
	assert.Empty(t, c.Allowlist) // an explicit [] is not the built-ins
}

func TestLoadMergesLocalOverride(t *testing.T) {
	dir := t.TempDir()
	mkDefaultDirs(t, dir)
	base := "cli: claude\ntmpfs:\n  - dist\nenv:\n  FOO: base\n  BAR: base\nallowlist:\n  - ccbox-defaults\nhost_git_config: true\n"
	local := "cli: codex\ntmpfs:\n  - build\nenv:\n  FOO: local\nallowlist:\n  - example.com\nhost_git_config: false\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte(base), ioutil.File))
	require.NoError(t, os.WriteFile(filepath.Join(dir, localFileName), []byte(local), ioutil.File))

	c, err := Load(dir, Config{})
	require.NoError(t, err)
	assert.Equal(t, harness.NameCodex, c.CLI)                                                 // scalar override, local wins
	assert.Equal(t, []string{".idea", ".vscode", "dist", "build"}, c.Tmpfs)                   // lists append, base first
	assert.Equal(t, map[string]string{"FOO": "local", "BAR": "base"}, c.Env)                  // env overlays, local wins
	assert.Equal(t, append(append([]string{}, allowDefaults...), "example.com"), c.Allowlist) // merged, then token expanded
	assert.Equal(t, new(false), c.HostGitConfig)                                              // scalar override, local wins
}

func TestLoadLocalOnly(t *testing.T) {
	dir := t.TempDir() // no base .ccbox.yaml
	require.NoError(t, os.WriteFile(filepath.Join(dir, localFileName),
		[]byte("allowlist:\n  - example.com\n"), ioutil.File))

	c, err := Load(dir, Config{})
	require.NoError(t, err)
	assert.Equal(t, []string{"example.com"}, c.Allowlist) // no base, no token → just the override
}

func TestLoadInvalidErrors(t *testing.T) {
	for _, name := range []string{fileName, localFileName} { // malformed yaml in either file errors
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("tmpfs: ["), ioutil.File))

			_, err := Load(dir, Config{})
			assert.Error(t, err)
		})
	}
}

func TestMergeIsPure(t *testing.T) {
	src := Config{
		CLI:           harness.NameClaude,
		Tmpfs:         []string{"dist"},
		Volumes:       []string{"target"},
		Env:           map[string]string{"FOO": "base", "BAR": "base"},
		Allowlist:     []string{"ccbox-defaults"},
		HostGitConfig: new(true),
	}
	other := Config{
		CLI:           harness.NameCodex,
		Tmpfs:         []string{"build"},
		Volumes:       []string{"cache"},
		Env:           map[string]string{"FOO": "local", "BAZ": "local"},
		Allowlist:     []string{"example.com"},
		HostGitConfig: new(false),
	}
	before := deepcopy.Of(src)

	got := src.merge(other)

	// merge must not mutate its receiver — every src field still equals its pre-merge snapshot
	srvVal, beforeVal, gotVal := reflect.ValueOf(src), reflect.ValueOf(before), reflect.ValueOf(got)
	for i := range srvVal.NumField() {
		field := srvVal.Type().Field(i).Name
		assert.Equal(t, beforeVal.Field(i).Interface(), srvVal.Field(i).Interface(), "merge mutated src.%s", field)
		assert.NotEqual(t, beforeVal.Field(i).Interface(), gotVal.Field(i).Interface(), "before = got on field %s, maybe missed a new field?", field)
	}

	// and the returned Config is the actual layering
	assert.Equal(t, []string{"dist", "build"}, got.Tmpfs)
	assert.Equal(t, []string{"target", "cache"}, got.Volumes)
	assert.Equal(t, map[string]string{"FOO": "local", "BAR": "base", "BAZ": "local"}, got.Env)
	assert.Equal(t, []string{"ccbox-defaults", "example.com"}, got.Allowlist)
	assert.Equal(t, new(false), got.HostGitConfig) // scalar: other (local) wins when set
	assert.Equal(t, harness.NameCodex, got.CLI)    // scalar: other (local) wins when set
}

func TestLoadRejectsUnknownCLI(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte("cli: emacs\n"), ioutil.File))

	_, err := Load(dir, Config{})
	assert.ErrorContains(t, err, "emacs")
}

func TestLoadFlagsOverrideFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte("cli: claude\n"), ioutil.File))
	require.NoError(t, os.WriteFile(filepath.Join(dir, localFileName), []byte("cli: codex\n"), ioutil.File))

	c, err := Load(dir, Config{CLI: harness.NameGrok})
	require.NoError(t, err)
	assert.Equal(t, harness.NameGrok, c.CLI) // a set flag wins over both files

	c, err = Load(dir, Config{})
	require.NoError(t, err)
	assert.Equal(t, harness.NameCodex, c.CLI) // an unset flag keeps the files' layering

	_, err = Load(dir, Config{CLI: "emacs"})
	assert.ErrorContains(t, err, "emacs") // the flag value validates like a file's
}
