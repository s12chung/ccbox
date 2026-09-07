package projectcfg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// TestMain defaults userConfigFile to a seeded temp file — prod parity: the user seed
// exists before Load runs — so tests don't read the developer's real user-level config;
// useUserConfigFile overrides per-test.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "projectcfg")
	if err != nil {
		panic(err)
	}
	userConfigFile = filepath.Join(dir, userFileName)
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
	useUserConfigFile(t, dir)

	path, err := SeedUserConfig("claude")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, userFileName), path)

	// the seed is the defaults' carrier
	want := Config{CLI: new("claude"), HostGitConfig: new(true), Tmpfs: tmpfsDefaults, Volumes: volumeDefaults, Allowlist: allowDefaults()}

	t.Run("seed alone resolves the default config", func(t *testing.T) {
		c, err := Load(dir, Config{})
		require.NoError(t, err)
		assert.Equal(t, want, c)
	})

	t.Run("an empty project file is unset like a missing one", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, projectFileName), nil, ioutil.File))

		c, err := Load(dir, Config{})
		require.NoError(t, err)
		assert.Equal(t, want, c)
	})
}

func TestSeedUserConfigSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, userFileName, "cli: codex\n")

	path, err := SeedUserConfig("claude")
	require.NoError(t, err)
	assert.Empty(t, path)

	// #nosec G304 -- the package's own temp user file
	body, err := os.ReadFile(userConfigFile)
	require.NoError(t, err)
	assert.Equal(t, "cli: codex\n", string(body)) // the user's own file is never touched
}

func TestSeedUserConfigChoosesCLI(t *testing.T) {
	dir := t.TempDir()
	useUserConfigFile(t, dir)

	path, err := SeedUserConfig("codex")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, userFileName), path)

	body, err := os.ReadFile(userConfigFile) // #nosec G304 -- the package's own temp user file
	require.NoError(t, err)
	assert.Contains(t, string(body), "cli: codex\n")
}

func TestUserConfigNeedsSeed(t *testing.T) {
	useUserConfigFile(t, t.TempDir())
	assert.True(t, UserConfigNeedsSeed())

	require.NoError(t, os.WriteFile(userConfigFile, []byte("cli: codex\n"), ioutil.File))
	assert.False(t, UserConfigNeedsSeed())
}

func TestLoadUnsetRequiredErrors(t *testing.T) {
	dir := t.TempDir()
	useUserConfigFile(t, t.TempDir()) // no user file

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
	assert.Equal(t, allowDefaults(), c.Allowlist) // lists append: [] adds nothing, the seed's token stays
}

func TestLoadExpanded(t *testing.T) {
	tests := []struct {
		name       string
		projectDir string
		body       string
		want       Config
	}{
		{
			"an all-nil config expands every list to the built-ins", t.TempDir(),
			"cli: claude\nhost_git_config: true\n",
			Config{
				CLI:           new("claude"),
				HostGitConfig: new(true),
				Tmpfs:         tmpfsDefaults,
				Volumes:       volumeDefaults,
				Allowlist:     allowDefaults(),
			},
		},
		{
			"the token expands in place, literals kept, repeats collapse", t.TempDir(),
			"cli: claude\nhost_git_config: true\ntmpfs:\n  - ccbox-defaults\n  - dist\n  - ccbox-defaults\n",
			Config{
				CLI:           new("claude"),
				HostGitConfig: new(true),
				Tmpfs:         append(append([]string{}, tmpfsDefaults...), "dist"),
				Volumes:       volumeDefaults,
				Allowlist:     allowDefaults(),
			},
		},
		{
			"explicit empty lists opt out", t.TempDir(),
			"cli: claude\nhost_git_config: true\nvolumes: []\n",
			Config{CLI: new("claude"), HostGitConfig: new(true), Tmpfs: tmpfsDefaults, Allowlist: allowDefaults()},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useUserConfigFile(t, t.TempDir()) // no user file: the project body stands alone
			writeConfig(t, tt.projectDir, projectFileName, tt.body)

			c, err := LoadExpanded(tt.projectDir, Config{})
			require.NoError(t, err)
			assert.Equal(t, tt.want, c)
		})
	}
}

func TestLoadMasks(t *testing.T) {
	// the project's absent dirs drop out, the token's defaults and own alike
	tests := []struct {
		name        string
		dirs        []string // present in the project dir
		body        string
		wantTmpfs   []string
		wantVolumes []string
	}{
		{
			"all absent drops out", nil,
			"tmpfs:\n  - dist\nvolumes:\n  - target\n",
			nil, nil,
		},
		{
			"present stays, absent drops",
			[]string{"dist"},
			"tmpfs:\n  - dist\n  - build\n",
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
			assert.Equal(t, tt.wantTmpfs, c.Tmpfs)
			assert.Equal(t, tt.wantVolumes, c.Volumes)
		})
	}
}

func TestNotFoundMasks(t *testing.T) {
	// present has the mask dirs on disk except .venv; absent has none
	present := t.TempDir()
	mkDirs(t, present, tmpfsDefaults...)
	mkDirs(t, present, "node_modules", "vendor/bundle")
	absent := t.TempDir()

	tests := []struct {
		name        string
		projectDir  string
		projectBody string
		wantTmpfs   []string
		wantVolumes []string
	}{
		{
			"the seed's token rejects the project's absent defaults", present,
			"",
			nil,
			[]string{".venv"},
		},
		{
			"the config's own absent dirs reject like the defaults", present,
			"tmpfs:\n  - dist\nvolumes:\n  - node_modules\n  - target\n",
			[]string{"dist"},
			[]string{".venv", "target"},
		},
		{
			"all absent rejects every entry, token and own alike", absent,
			"tmpfs:\n  - dist\n",
			append(append([]string{}, tmpfsDefaults...), "dist"),
			volumeDefaults,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.projectBody != "" {
				require.NoError(t, os.WriteFile(filepath.Join(tt.projectDir, projectFileName), []byte(tt.projectBody), ioutil.File))
			}
			tmpfs, volumes, err := NotFoundMasks(tt.projectDir, Config{})
			require.NoError(t, err)
			assert.Equal(t, tt.wantTmpfs, tmpfs)
			assert.Equal(t, tt.wantVolumes, volumes)
		})
	}
}

func TestInitWritesLoadableDefault(t *testing.T) {
	dir := t.TempDir()
	path, err := Init(dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, projectFileName), path)

	// the written scaffold must resolve exactly like no file at all
	c, err := Load(dir, Config{})
	require.NoError(t, err)
	fresh, err := Load(t.TempDir(), Config{})
	require.NoError(t, err)
	assert.Equal(t, fresh, c)
}

func TestInitRefusesExisting(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectFileName), []byte("tmpfs: []\n"), ioutil.File))

	_, err := Init(dir)
	assert.ErrorIs(t, err, seed.ErrExists)
}

// TestRenderConfigFixtures pins the render output to the committed testdata fixtures,
// which `make lint` yq-checks as YAML. Regenerate with:
// `UPDATE_FIXTURES=1 go test ./pkg/projectcfg/ -run TestRenderConfigFixtures`
// (an env var, not a flag: the go tool doesn't forward -update reliably)
func TestRenderConfigFixtures(t *testing.T) {
	for _, tt := range []struct {
		name string // fixture name: testdata/<name>.ccbox.yaml
		c    Config
	}{
		{"init", Config{}},                      // `ccbox config init`'s fill-in scaffold
		{"user-seed", userSeedConfig("claude")}, // the safe-seeded user-level config
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderConfig(tt.c)
			require.NoError(t, err)

			path := filepath.Join("testdata", tt.name+".ccbox.yaml")
			if os.Getenv("UPDATE_FIXTURES") != "" {
				require.NoError(t, os.MkdirAll(filepath.Dir(path), ioutil.Dir))
				require.NoError(t, os.WriteFile(path, []byte(got), ioutil.File))
			}

			// #nosec G304 -- the package's own fixture path
			want, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, string(want), got)
		})
	}
}

func TestAllowDefaultsIncludeEveryCli(t *testing.T) {
	// every loaded cli's domains count — embedded or user-defined, they all sit in All()
	for _, c := range harness.All() {
		for _, d := range c.AllowDomains {
			assert.Containsf(t, allowDefaults(), d, "%s: %s", c.Name, d)
		}
	}
}

func TestLoadLayersFiles(t *testing.T) {
	dir := t.TempDir()
	mkDirs(t, dir, tmpfsDefaults...)
	mkDirs(t, dir, "dist", "build", "cache") // the layers' own mask dirs must exist to survive
	bodies := map[string]string{             // one entry per configFiles term
		"user":    "cli: grok\ntmpfs:\n  - ccbox-defaults\n  - dist\nenv:\n  FOO: user\n  BAR: user\nallowlist:\n  - user.example.dev\n",
		"project": "cli: claude\ntmpfs:\n  - build\nenv:\n  FOO: project\n  BAZ: project\nallowlist:\n  - ccbox-defaults\nhost_git_config: true\n",
		"local":   "cli: codex\ntmpfs:\n  - cache\nenv:\n  FOO: local\nallowlist:\n  - example.com\nhost_git_config: false\n",
	}
	for _, cf := range configFiles {
		writeConfig(t, dir, cf.name, bodies[cf.term])
	}

	c, err := Load(dir, Config{})
	require.NoError(t, err)
	assert.Equal(t, "codex", *c.CLI)             // later layer wins
	assert.Equal(t, new(false), c.HostGitConfig) // later layer wins

	// the user layer's tmpfs token expands in place; lists append, lowest layer first, dedupe collapses
	assert.Equal(t, []string{".idea", ".vscode", "dist", "build", "cache"}, c.Tmpfs)

	// env overlays, later wins
	assert.Equal(t, map[string]string{"FOO": "local", "BAR": "user", "BAZ": "project"}, c.Env)

	// lists append, the token expands in place
	assert.Equal(t, append(append([]string{"user.example.dev"}, allowDefaults()...), "example.com"), c.Allowlist)
}

func TestLoadSingleFileOnly(t *testing.T) {
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
				assert.Equal(t, append(allowDefaults(), "example.com"), c.Allowlist)
			}
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
		CLI:           new("claude"),
		Tmpfs:         []string{"dist"},
		Volumes:       []string{"target"},
		Env:           map[string]string{"FOO": "base", "BAR": "base"},
		Allowlist:     []string{"ccbox-defaults"},
		HostGitConfig: new(true),
	}
	other := Config{
		CLI:           new("codex"),
		Tmpfs:         []string{"build"},
		Volumes:       []string{"cache"},
		Env:           map[string]string{"FOO": "local", "BAZ": "local"},
		Allowlist:     []string{"example.com"},
		HostGitConfig: new(false),
	}

	got := src.merge(other)

	assert.Equal(t, deepcopy.Of(src), src) // merge never mutates its receiver
	assert.Equal(t, []string{"dist", "build"}, got.Tmpfs)
	assert.Equal(t, []string{"target", "cache"}, got.Volumes)
	assert.Equal(t, map[string]string{"FOO": "local", "BAR": "base", "BAZ": "local"}, got.Env)
	assert.Equal(t, []string{"ccbox-defaults", "example.com"}, got.Allowlist)
	assert.Equal(t, new(false), got.HostGitConfig) // a set later layer wins
	assert.Equal(t, "codex", *got.CLI)             // a set later layer wins
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"unknown cli", "cli: emacs\n", []string{"CLI", "is not one of [claude codex grok opencode]"}},
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

	c, err := Load(dir, Config{CLI: new("grok")})
	require.NoError(t, err)
	assert.Equal(t, "grok", *c.CLI) // a set flag wins over both files

	c, err = Load(dir, Config{})
	require.NoError(t, err)
	assert.Equal(t, "codex", *c.CLI) // an unset flag keeps the files' layering

	_, err = Load(dir, Config{CLI: new("emacs")})
	assert.ErrorContains(t, err, "is not one of") // the flag value validates like a file's
}
