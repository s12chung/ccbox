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

// useUserConfigFile points userConfigFile at dir/ccbox.yaml, restoring after the test.
func useUserConfigFile(t *testing.T, dir string) {
	t.Helper()
	saved := userConfigFile
	t.Cleanup(func() { userConfigFile = saved })
	userConfigFile = filepath.Join(dir, userFileName)
}

// writeConfig writes body to the named config file in dir, wiring the user file to its
// own temp dir. Returns the path written.
func writeConfig(t *testing.T, dir string, name string, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if name == userFileName {
		useUserConfigFile(t, t.TempDir())
		path = userConfigFile
	}
	require.NoError(t, os.WriteFile(path, []byte(body), ioutil.File))
	return path
}

// configFiles lists the config files by level term and file name, in load order
// (lowest precedence first)
var configFiles = []struct {
	term string // user, project, local
	name string
}{
	{"user", userFileName},
	{"project", projectFileName},
	{"local", localFileName},
}

// TestMain defaults userConfigFile to a nonexistent temp path, so tests don't read the
// developer's real user-level config; useUserConfigFile overrides per-test.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "projectcfg")
	if err != nil {
		panic(err)
	}
	userConfigFile = filepath.Join(dir, userFileName)
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func TestLoadParses(t *testing.T) {
	dir := t.TempDir()
	mkDefaultDirs(t, dir)
	body := "cli: codex\ntmpfs:\n  - dist\n  - build\nenv:\n  FOO: bar\n" +
		"allowlist:\n  - ccbox-defaults\n  - example.com\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectFileName), []byte(body), ioutil.File))

	c, err := Load(dir, Config{})
	require.NoError(t, err)
	assert.Equal(t, "codex", c.CLI)
	assert.Equal(t, []string{".idea", ".vscode", "dist", "build"}, c.Tmpfs) // present defaults prepended
	assert.Equal(t, map[string]string{"FOO": "bar"}, c.Env)
	assert.Equal(t, append(allowDefaults(), "example.com"), c.Allowlist) // token expanded
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
				require.NoError(t, os.WriteFile(filepath.Join(dir, projectFileName), []byte(""), ioutil.File))
			}
			c, err := Load(dir, Config{})
			require.NoError(t, err)
			assert.Equal(t, "claude", c.CLI)              // unset cli → claude
			assert.Equal(t, tmpfsDefaults, c.Tmpfs)       // present always-on masks
			assert.Equal(t, allowDefaults(), c.Allowlist) // unset allowlist → the built-ins
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
	assert.Equal(t, filepath.Join(dir, projectFileName), path)

	c, err := Load(dir, Config{}) // the written scaffold must resolve to the no-file behavior
	require.NoError(t, err)
	assert.Equal(t, Defaulted(dir, Config{}).Tmpfs, c.Tmpfs)
	assert.Equal(t, Defaulted(dir, Config{}).Allowlist, c.Allowlist)
	assert.Empty(t, c.Env)
}

func TestInitRefusesExisting(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectFileName), []byte("tmpfs: []\n"), ioutil.File))

	_, err := Init(dir)
	assert.ErrorContains(t, err, "already exists")
}

func TestDefaultedAllowlistNilVsEmpty(t *testing.T) {
	dir := t.TempDir()
	assert.Equal(t, allowDefaults(), Defaulted(dir, Config{Allowlist: nil}).Allowlist) // unset → built-ins
	assert.Empty(t, Defaulted(dir, Config{Allowlist: []string{}}).Allowlist)           // explicit [] → nothing
}

func TestLoadEmptyAllowlistAllowsNothing(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectFileName), []byte("allowlist: []\n"), ioutil.File))

	c, err := Load(dir, Config{})
	require.NoError(t, err)
	assert.Empty(t, c.Allowlist) // an explicit [] is not the built-ins
}

func TestAllowDefaultsIncludeEveryCli(t *testing.T) {
	// every loaded cli's domains count — embedded or user-defined, they all sit in All()
	for _, c := range harness.All() {
		for _, d := range c.AllowDomains {
			assert.Containsf(t, allowDefaults(), d, "%s: %s", c.Name, d)
			assert.Containsf(t, Defaulted(t.TempDir(), Config{}).Allowlist, d, "%s: %s", c.Name, d)
		}
	}
}

func TestLoadLayersFiles(t *testing.T) {
	dir := t.TempDir()
	mkDefaultDirs(t, dir)
	bodies := map[string]string{ // one entry per configFiles term
		"user":    "cli: grok\ntmpfs:\n  - dist\nenv:\n  FOO: user\n  BAR: user\nallowlist:\n  - user.example.dev\n",
		"project": "cli: claude\ntmpfs:\n  - build\nenv:\n  FOO: project\n  BAZ: project\nallowlist:\n  - ccbox-defaults\nhost_git_config: true\n",
		"local":   "cli: codex\ntmpfs:\n  - cache\nenv:\n  FOO: local\nallowlist:\n  - example.com\nhost_git_config: false\n",
	}
	for _, cf := range configFiles {
		writeConfig(t, dir, cf.name, bodies[cf.term])
	}

	c, err := Load(dir, Config{})
	require.NoError(t, err)
	assert.Equal(t, "codex", c.CLI)                                                                               // scalars: later layer wins
	assert.Equal(t, []string{".idea", ".vscode", "dist", "build", "cache"}, c.Tmpfs)                              // lists append, lowest layer first
	assert.Equal(t, map[string]string{"FOO": "local", "BAR": "user", "BAZ": "project"}, c.Env)                    // env overlays, later wins
	assert.Equal(t, append(append([]string{"user.example.dev"}, allowDefaults()...), "example.com"), c.Allowlist) // lists append, token expands in place
	assert.Equal(t, new(false), c.HostGitConfig)                                                                  // scalars: later layer wins
}

func TestLoadSingleFileOnly(t *testing.T) {
	for _, cf := range configFiles { // the other files are absent
		t.Run(cf.term, func(t *testing.T) {
			dir := t.TempDir()
			writeConfig(t, dir, cf.name, "cli: grok\nallowlist:\n  - example.com\n")

			c, err := Load(dir, Config{})
			require.NoError(t, err)
			assert.Equal(t, "grok", c.CLI)
			assert.Equal(t, []string{"example.com"}, c.Allowlist) // no token → no defaults pulled in
		})
	}
}

func TestLoadInvalidErrors(t *testing.T) {
	for _, cf := range configFiles { // malformed yaml in any file errors
		t.Run(cf.term, func(t *testing.T) {
			dir := t.TempDir()
			path := writeConfig(t, dir, cf.name, "tmpfs: [")

			_, err := Load(dir, Config{})
			require.Error(t, err)
			assert.ErrorContains(t, err, path) // the error names the offending file
		})
	}
}

func TestMergeIsPure(t *testing.T) {
	src := Config{
		CLI:           "claude",
		Tmpfs:         []string{"dist"},
		Volumes:       []string{"target"},
		Env:           map[string]string{"FOO": "base", "BAR": "base"},
		Allowlist:     []string{"ccbox-defaults"},
		HostGitConfig: new(true),
	}
	other := Config{
		CLI:           "codex",
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
	assert.Equal(t, new(false), got.HostGitConfig) // scalar: other (the later layer) wins when set
	assert.Equal(t, "codex", got.CLI)              // scalar: other (the later layer) wins when set
}

func TestLoadRejectsUnknownCLI(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectFileName), []byte("cli: emacs\n"), ioutil.File))

	_, err := Load(dir, Config{})
	require.ErrorContains(t, err, "is not one of [claude codex grok opencode]")
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"absolute tmpfs", "tmpfs:\n  - /etc\n", []string{"Tmpfs", "Match"}},
		{"tmpfs traversal", "tmpfs:\n  - ../escape\n", []string{"Tmpfs", "Match"}},
		{"absolute volume", "volumes:\n  - /var\n", []string{"Volumes", "Match"}},
		{"bad env key", "env:\n  bad-key: \"1\"\n", []string{"Env", "Match"}},
		{"empty env value", "env:\n  FOO: \"\"\n", []string{"Env", "Present"}},
		{"bad allow domain", "allowlist:\n  - \"https://x.dev\"\n", []string{"Allowlist", "Match"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, projectFileName), []byte(tt.body), ioutil.File))

			_, err := Load(dir, Config{})
			require.Error(t, err)
			for _, want := range tt.want {
				assert.ErrorContains(t, err, want)
			}
		})
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	for _, cf := range configFiles {
		t.Run(cf.term, func(t *testing.T) {
			dir := t.TempDir()
			if cf.term == "local" { // empty project file so the error attributes to local
				writeConfig(t, dir, projectFileName, "")
			}
			writeConfig(t, dir, cf.name, "cli: claude\nbogus: true\n")

			_, err := Load(dir, Config{})
			require.ErrorContains(t, err, "field bogus not found")
			assert.ErrorContains(t, err, cf.name) // the error names the offending file
		})
	}
}

func TestLoadFlagsOverrideFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectFileName), []byte("cli: claude\n"), ioutil.File))
	require.NoError(t, os.WriteFile(filepath.Join(dir, localFileName), []byte("cli: codex\n"), ioutil.File))

	c, err := Load(dir, Config{CLI: "grok"})
	require.NoError(t, err)
	assert.Equal(t, "grok", c.CLI) // a set flag wins over both files

	c, err = Load(dir, Config{})
	require.NoError(t, err)
	assert.Equal(t, "codex", c.CLI) // an unset flag keeps the files' layering

	_, err = Load(dir, Config{CLI: "emacs"})
	assert.ErrorContains(t, err, "is not one of") // the flag value validates like a file's
}
