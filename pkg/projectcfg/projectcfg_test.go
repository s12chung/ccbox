package projectcfg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/util/deepcopy"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/seed"
)

// mkDirs creates the given project-relative dirs under dir, so presence-sensitive
// defaults pick them up.
func mkDirs(t *testing.T, dir string, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, d), ioutil.Dir))
	}
}

// useHome points home at a fresh temp tree, restoring it after the test, and creates
// the user config's dir. Returns the user config's path.
func useHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".ccbox", "config"), ioutil.Dir))
	return UserConfigFile()
}

// writeConfig writes body to the named config file in dir, wiring the user file to its
// own temp home. Returns the path written.
func writeConfig(t *testing.T, dir string, name string, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if name == userFileName {
		path = useHome(t)
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

// TestMain points home at a temp tree and seeds the user config — prod parity: the
// user seed exists before Load runs — so tests don't read the developer's real
// user-level config; useHome overrides per-test.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "projectcfg")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("HOME", dir); err != nil {
		panic(err)
	}
	if _, err := SeedUserConfig("claude"); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func TestSeedUserConfig(t *testing.T) {
	dir := t.TempDir()
	mkDirs(t, dir, tmpfsDefaults...)
	mkDirs(t, dir, volumeDefaults...)
	useHome(t)

	path, err := SeedUserConfig("claude")
	require.NoError(t, err)
	assert.Equal(t, UserConfigFile(), path)

	// the seed is the defaults' carrier
	want := Config{
		CLI:           new("claude"),
		HostGitConfig: new(true),
		TmpfsMasks:    []string{DefaultsToken},
		VolumeMasks:   []string{DefaultsToken},
		Allowlist:     []string{DefaultsToken},

		projectDir: dir,
	}

	t.Run("seed alone loads the default config", func(t *testing.T) {
		c, err := Load(dir, Config{})
		require.NoError(t, err)
		assert.Equal(t, &want, c)
	})

	t.Run("an empty project file is unset like a missing one", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, projectFileName), nil, ioutil.File))

		c, err := Load(dir, Config{})
		require.NoError(t, err)
		assert.Equal(t, &want, c)
	})
}

func TestSeedUserConfigSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, userFileName, "cli: codex\n")

	path, err := SeedUserConfig("claude")
	require.NoError(t, err)
	assert.Empty(t, path)

	// #nosec G304 -- the package's own temp user file
	body, err := os.ReadFile(UserConfigFile())
	require.NoError(t, err)
	assert.Equal(t, "cli: codex\n", string(body)) // the user's own file is never touched
}

func TestSeedUserConfigChoosesCLI(t *testing.T) {
	useHome(t)

	path, err := SeedUserConfig("codex")
	require.NoError(t, err)
	assert.Equal(t, UserConfigFile(), path)

	body, err := os.ReadFile(UserConfigFile()) // #nosec G304 -- the package's own temp user file
	require.NoError(t, err)
	assert.Contains(t, string(body), "cli: codex\n")
}

func TestLoadUnsetRequiredErrors(t *testing.T) {
	dir := t.TempDir()
	useHome(t) // no user file

	_, err := Load(dir, Config{})
	require.Error(t, err)
	require.ErrorContains(t, err, "CLI.Nil: CLI is nil")
	assert.ErrorContains(t, err, "HostGitConfig.Nil: HostGitConfig is nil")
}

func TestLoadEmptyListKeepsLowerLayers(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectFileName), []byte("allowlist: []\n"), ioutil.File))

	c, err := Load(dir, Config{})
	require.NoError(t, err)
	assert.Equal(t, []string{DefaultsToken}, c.Allowlist) // lists append: [] adds nothing, the seed's token stays
}

func TestConfig_ListsExpanded(t *testing.T) {
	tests := []struct {
		name                string
		projectDir          string
		body                string
		want                Config
		wantTmpfsExpanded   []string
		wantVolumesExpanded []string
	}{
		{
			"an all-nil config expands every list to nothing", t.TempDir(),
			"cli: claude\nhost_git_config: true\n",
			Config{
				CLI:           new("claude"),
				HostGitConfig: new(true),
			},
			nil, nil,
		},
		{
			"the token expands in place, literals kept, repeats collapse", t.TempDir(),
			"cli: claude\nhost_git_config: true\ntmpfsMasks:\n  - ccbox-defaults\n  - dist\n  - ccbox-defaults\n",
			Config{
				CLI:           new("claude"),
				HostGitConfig: new(true),
				TmpfsMasks:    []string{DefaultsToken, "dist", DefaultsToken},
			},
			append(append([]string{}, tmpfsDefaults...), "dist"),
			nil,
		},
		{
			"an explicit empty list stays empty like an unset one", t.TempDir(),
			"cli: claude\nhost_git_config: true\nvolumeMasks: []\n",
			Config{CLI: new("claude"), HostGitConfig: new(true), VolumeMasks: []string{}},
			nil, nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useHome(t) // no user file: the project body stands alone
			writeConfig(t, tt.projectDir, projectFileName, tt.body)
			tt.want.projectDir = tt.projectDir

			c, err := Load(tt.projectDir, Config{})
			require.NoError(t, err)
			assert.Equal(t, &tt.want, c)
			assert.Equal(t, tt.wantTmpfsExpanded, c.TmpfsMasksExpanded())
			assert.Equal(t, tt.wantVolumesExpanded, c.VolumeMasksExpanded())
		})
	}
}

func TestLoad_StoresProjectDir(t *testing.T) {
	dir := t.TempDir()
	useHome(t) // no user file
	writeConfig(t, dir, projectFileName, "cli: claude\nhost_git_config: true\n")

	c, err := Load(dir, Config{})
	require.NoError(t, err)
	assert.Equal(t, dir, c.ProjectDir())
}

func TestAccessorsCache(t *testing.T) {
	dir := t.TempDir()
	useHome(t) // no user file: no defaults token
	mkDirs(t, dir, "dist")
	writeConfig(t, dir, projectFileName, "cli: claude\nhost_git_config: true\ntmpfsMasks:\n  - dist\n")

	c, err := Load(dir, Config{})
	require.NoError(t, err)

	expanded := c.TmpfsMasksExpanded()
	present := c.TmpfsMasksPresent()
	assert.Equal(t, []string{"dist"}, expanded)
	assert.Equal(t, []string{"dist"}, present)

	// later raw changes don't leak through the cached resolutions
	c.TmpfsMasks = append(c.TmpfsMasks, "build")
	assert.Equal(t, expanded, c.TmpfsMasksExpanded())
	assert.Equal(t, present, c.TmpfsMasksPresent())
}

func TestLoadedPaths(t *testing.T) {
	tests := []struct {
		name  string
		files []string // config file names present on disk, in load order
	}{
		{"no files", nil},
		{"project only", []string{projectFileName}},
		{"user and local", []string{userFileName, localFileName}},
		{"all three", []string{userFileName, projectFileName, localFileName}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useHome(t) // no user file unless written below
			dir := t.TempDir()
			for _, name := range tt.files {
				writeConfig(t, dir, name, "cli: claude\n")
			}
			var want []string
			for _, name := range tt.files {
				if name == userFileName {
					want = append(want, UserConfigFile())
					continue
				}
				want = append(want, filepath.Join(dir, name))
			}
			assert.Equal(t, want, LoadedPaths(dir))
		})
	}

	t.Run("an empty file counts as loaded", func(t *testing.T) {
		useHome(t)
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, projectFileName), nil, ioutil.File))

		assert.Equal(t, []string{filepath.Join(dir, projectFileName)}, LoadedPaths(dir))
	})
}

func TestConfig_MasksPresent(t *testing.T) {
	tests := []struct {
		name            string
		dirs            []string // present in the project dir
		body            string
		wantTmpfsMasks  []string
		wantVolumeMasks []string
	}{
		{
			"all absent drops out", nil,
			"tmpfsMasks:\n  - dist\nvolumeMasks:\n  - target\n",
			nil, nil,
		},
		{
			"present stays, absent drops",
			[]string{"dist"},
			"tmpfsMasks:\n  - dist\n  - build\n",
			[]string{"dist"},
			nil, // the seed's defaults are all absent here
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			mkDirs(t, dir, tt.dirs...)
			writeConfig(t, dir, projectFileName, tt.body)

			c, err := Load(dir, Config{})
			require.NoError(t, err)
			assert.Equal(t, tt.wantTmpfsMasks, c.TmpfsMasksPresent())
			assert.Equal(t, tt.wantVolumeMasks, c.VolumeMasksPresent())
		})
	}
}

func TestConfig_MasksAbsent(t *testing.T) {
	// present has the mask dirs on disk except .venv; absent has none
	present := t.TempDir()
	mkDirs(t, present, tmpfsDefaults...)
	mkDirs(t, present, "node_modules", "vendor/bundle")
	absent := t.TempDir()

	tests := []struct {
		name            string
		projectDir      string
		projectBody     string
		wantTmpfsMasks  []string
		wantVolumeMasks []string
	}{
		{
			"the seed's token rejects the project's absent defaults", present,
			"",
			nil,
			[]string{".venv"},
		},
		{
			"the config's own absent dirs reject like the defaults", present,
			"tmpfsMasks:\n  - dist\nvolumeMasks:\n  - node_modules\n  - target\n",
			[]string{"dist"},
			[]string{".venv", "target"},
		},
		{
			"all absent rejects every entry, token and own alike", absent,
			"tmpfsMasks:\n  - dist\n",
			append(append([]string{}, tmpfsDefaults...), "dist"),
			volumeDefaults,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.projectBody != "" {
				require.NoError(t, os.WriteFile(filepath.Join(tt.projectDir, projectFileName), []byte(tt.projectBody), ioutil.File))
			}
			c, err := Load(tt.projectDir, Config{})
			require.NoError(t, err)
			assert.Equal(t, tt.wantTmpfsMasks, c.TmpfsMasksAbsent())
			assert.Equal(t, tt.wantVolumeMasks, c.VolumeMasksAbsent())
		})
	}
}

func TestInit_WritesLoadableDefault(t *testing.T) {
	dir := t.TempDir()
	path, err := Init(dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, projectFileName), path)

	// projectDir is the only Config difference
	c, err := Load(dir, Config{})
	require.NoError(t, err)
	fresh, err := Load(t.TempDir(), Config{})
	require.NoError(t, err)
	cOut, err := yaml.Marshal(c)
	require.NoError(t, err)
	freshOut, err := yaml.Marshal(fresh)
	require.NoError(t, err)
	assert.Equal(t, string(freshOut), string(cOut))
}

func TestInit_RefusesExisting(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectFileName), []byte("tmpfsMasks: []\n"), ioutil.File))

	_, err := Init(dir)
	assert.ErrorIs(t, err, seed.ErrExists)
}

func TestAllowDefaultsIncludeEveryCli(t *testing.T) {
	// every loaded cli's domains count — embedded or user-defined, they all sit in All()
	for _, c := range harness.All() {
		for _, d := range c.AllowDomains {
			assert.Containsf(t, AllowDefaults(), d, "%s: %s", c.Name, d)
		}
	}
}

func TestLoad_LayersFiles(t *testing.T) {
	dir := t.TempDir()
	mkDirs(t, dir, tmpfsDefaults...)
	mkDirs(t, dir, "dist", "build", "cache") // the layers' own mask dirs must exist to survive
	bodies := map[string]string{             // one entry per configFiles term
		"user":    "cli: grok\ntmpfsMasks:\n  - ccbox-defaults\n  - dist\nenv:\n  FOO: user\n  BAR: user\nallowlist:\n  - user.example.dev\n",
		"project": "cli: claude\ntmpfsMasks:\n  - build\nenv:\n  FOO: project\n  BAZ: project\nallowlist:\n  - ccbox-defaults\nhost_git_config: true\n",
		"local":   "cli: codex\ntmpfsMasks:\n  - cache\nenv:\n  FOO: local\nallowlist:\n  - example.com\nhost_git_config: false\n",
	}
	for _, cf := range configFiles {
		writeConfig(t, dir, cf.name, bodies[cf.term])
	}

	c, err := Load(dir, Config{})
	require.NoError(t, err)
	assert.Equal(t, "codex", *c.CLI)             // later layer wins
	assert.Equal(t, new(false), c.HostGitConfig) // later layer wins

	// lists append raw, lowest layer first, the seed's tokens carried as-is
	assert.Equal(t, []string{DefaultsToken, "dist", "build", "cache"}, c.TmpfsMasks)

	// env overlays, later wins
	assert.Equal(t, map[string]string{"FOO": "local", "BAR": "user", "BAZ": "project"}, c.Env)

	assert.Equal(t, []string{"user.example.dev", DefaultsToken, "example.com"}, c.Allowlist)
}

func TestLoad_SingleFileOnly(t *testing.T) {
	for _, cf := range configFiles { // the other files are absent
		t.Run(cf.term, func(t *testing.T) {
			dir := t.TempDir()
			writeConfig(t, dir, cf.name, "cli: grok\nhost_git_config: true\nallowlist:\n  - example.com\n")

			c, err := Load(dir, Config{})
			require.NoError(t, err)
			assert.Equal(t, "grok", *c.CLI)
			if cf.term == "user" { // the user file replaces the seed wholesale
				assert.Equal(t, []string{"example.com"}, c.Allowlist) // no token → no defaults pulled in
			} else { // the user seed's token still underlies the project/local file
				assert.Equal(t, []string{DefaultsToken, "example.com"}, c.Allowlist)
			}
		})
	}
}

func TestLoad_InvalidErrors(t *testing.T) {
	for _, cf := range configFiles { // malformed yaml in any file errors
		t.Run(cf.term, func(t *testing.T) {
			dir := t.TempDir()
			path := writeConfig(t, dir, cf.name, "tmpfsMasks: [")

			_, err := Load(dir, Config{})
			require.Error(t, err)
			assert.ErrorContains(t, err, path) // the error names the offending file
		})
	}
}

func TestConfig_mergeOverlays(t *testing.T) {
	c := Config{
		CLI:           new("claude"),
		TmpfsMasks:    []string{"dist"},
		VolumeMasks:   []string{"target"},
		Env:           map[string]string{"FOO": "base", "BAR": "base"},
		Allowlist:     []string{"ccbox-defaults"},
		HostGitConfig: new(true),
	}
	other := Config{
		CLI:           new("codex"),
		TmpfsMasks:    []string{"build"},
		VolumeMasks:   []string{"cache"},
		Env:           map[string]string{"FOO": "local", "BAZ": "local"},
		Allowlist:     []string{"example.com"},
		HostGitConfig: new(false),
	}

	c.merge(other)

	assert.Equal(t, deepcopy.Of(other), other) // merging never touches the later layer
	assert.Equal(t, []string{"dist", "build"}, c.TmpfsMasks)
	assert.Equal(t, []string{"target", "cache"}, c.VolumeMasks)
	assert.Equal(t, map[string]string{"FOO": "local", "BAR": "base", "BAZ": "local"}, c.Env)
	assert.Equal(t, []string{"ccbox-defaults", "example.com"}, c.Allowlist)
	assert.Equal(t, new(false), c.HostGitConfig) // a set later layer wins
	assert.Equal(t, "codex", *c.CLI)             // a set later layer wins
}

func TestLoad_RejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"unknown cli", "cli: emacs\n", []string{"CLI", "is not one of [claude codex grok opencode]"}},
		{"absolute tmpfsMasks", "tmpfsMasks:\n  - /etc\n", []string{"TmpfsMasks", "Match"}},
		{"tmpfsMasks traversal", "tmpfsMasks:\n  - ../escape\n", []string{"TmpfsMasks", "Match"}},
		{"absolute volumeMasks", "volumeMasks:\n  - /var\n", []string{"VolumeMasks", "Match"}},
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

	c, err := Load(dir, Config{CLI: new("grok")})
	require.NoError(t, err)
	assert.Equal(t, "grok", *c.CLI) // a set flag wins over both files

	c, err = Load(dir, Config{})
	require.NoError(t, err)
	assert.Equal(t, "codex", *c.CLI) // an unset flag keeps the files' layering

	_, err = Load(dir, Config{CLI: new("emacs")})
	assert.ErrorContains(t, err, "is not one of") // the flag value validates like a file's
}
