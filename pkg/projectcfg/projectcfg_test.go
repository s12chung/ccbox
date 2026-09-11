package projectcfg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/s12chung/ccbox/pkg/harness"
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
	if name == userConfigFileName {
		path = useHome(t)
	}
	require.NoError(t, os.WriteFile(path, []byte(body), ioutil.File))
	return path
}

func loadProjectConfigYAML(t *testing.T, projectDir string) string {
	t.Helper()
	c, err := Load(projectDir, Config{})
	require.NoError(t, err)
	out, err := yaml.Marshal(c)
	require.NoError(t, err)
	return string(out)
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	body, err := os.ReadFile(src) // #nosec G304 -- the test's own scaffold path
	require.NoError(t, err)
	// #nosec G703 -- the scaffold re-written verbatim into the test's own temp project
	require.NoError(t, os.WriteFile(dst, body, ioutil.File))
}

// configFiles lists the config files by level term and file name, in load order
// (lowest precedence first)
var configFiles = []struct {
	term string // user, project, local
	name string
}{
	{"user", userConfigFileName},
	{"project", projectConfigFileName},
	{"local", localConfigFileName},
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
		CLIName:       new("claude"),
		HostGitConfig: new(true),
		TmpfsMasks:    []string{DefaultsToken},
		VolumeMasks:   []string{DefaultsToken},
		ReadOnlyGlobs: []string{DefaultsToken},
		Allowlist:     []string{DefaultsToken},

		projectDir: dir,
	}

	t.Run("seed alone loads the default config", func(t *testing.T) {
		c, err := Load(dir, Config{})
		require.NoError(t, err)
		assert.Equal(t, &want, c)
	})

	t.Run("an empty project file is unset like a missing one", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, projectConfigFileName), nil, ioutil.File))

		c, err := Load(dir, Config{})
		require.NoError(t, err)
		assert.Equal(t, &want, c)
	})
}

func TestSeedUserConfig_SkipsExisting(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, userConfigFileName, "cli: codex\n")

	path, err := SeedUserConfig("claude")
	require.NoError(t, err)
	assert.Empty(t, path)

	// #nosec G304 -- the package's own temp user file
	body, err := os.ReadFile(UserConfigFile())
	require.NoError(t, err)
	assert.Equal(t, "cli: codex\n", string(body)) // the user's own file is never touched
}

func TestSeedUserConfig_ChoosesCLI(t *testing.T) {
	useHome(t)

	path, err := SeedUserConfig("codex")
	require.NoError(t, err)
	assert.Equal(t, UserConfigFile(), path)

	body, err := os.ReadFile(UserConfigFile()) // #nosec G304 -- the package's own temp user file
	require.NoError(t, err)
	assert.Contains(t, string(body), "cli: codex\n")
}

func TestLoadedPaths(t *testing.T) {
	tests := []struct {
		name  string
		files []string // config file names present on disk, in load order
	}{
		{"no files", nil},
		{"project only", []string{projectConfigFileName}},
		{"user and local", []string{userConfigFileName, localConfigFileName}},
		{"all three", []string{userConfigFileName, projectConfigFileName, localConfigFileName}},
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
				if name == userConfigFileName {
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
		require.NoError(t, os.WriteFile(filepath.Join(dir, projectConfigFileName), nil, ioutil.File))

		assert.Equal(t, []string{filepath.Join(dir, projectConfigFileName)}, LoadedPaths(dir))
	})
}

func TestInit_WritesLoadableDefault(t *testing.T) {
	projectDir := t.TempDir()
	configPath, err := Init(projectDir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(projectDir, projectConfigFileName), configPath)

	cOut := loadProjectConfigYAML(t, projectDir)

	freshDir := t.TempDir()
	copyFile(t, configPath, filepath.Join(freshDir, projectConfigFileName))
	freshOut := loadProjectConfigYAML(t, freshDir)

	assert.Equal(t, freshOut, cOut)
}

func TestInit_RefusesExisting(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectConfigFileName), []byte("tmpfs_masks: []\n"), ioutil.File))

	_, err := Init(dir)
	assert.ErrorIs(t, err, seed.ErrExists)
}

func TestAllowDefaults_IncludeEveryCli(t *testing.T) {
	// every loaded cli's domains count — embedded or user-defined, they all sit in All()
	for _, c := range harness.All() {
		for _, d := range c.AllowDomains {
			assert.Containsf(t, AllowDefaults(), d, "%s: %s", c.Name, d)
		}
	}
}

func TestLoad_UnsetRequiredErrors(t *testing.T) {
	dir := t.TempDir()
	useHome(t) // no user file

	_, err := Load(dir, Config{})
	require.Error(t, err)
	require.ErrorContains(t, err, "CLIName.Nil: CLIName is nil")
	assert.ErrorContains(t, err, "HostGitConfig.Nil: HostGitConfig is nil")
}

func TestLoad_HostGitConfigUnresolvable(t *testing.T) {
	dir := t.TempDir()

	t.Run("enabled errors", func(t *testing.T) {
		writeConfig(t, dir, projectConfigFileName, "cli: claude\nhost_git_config: true\n")
		file := filepath.Join(t.TempDir(), "not-a-dir")
		require.NoError(t, os.WriteFile(file, nil, ioutil.File))
		t.Setenv("XDG_CONFIG_HOME", file) // a file, not a dir: stat <file>/git errors

		_, err := Load(dir, Config{})
		require.Error(t, err)
		require.ErrorContains(t, err, "HostGitConfig.HasValidGitDir")
		assert.ErrorContains(t, err, "not a directory")
	})

	t.Run("disabled loads", func(t *testing.T) {
		writeConfig(t, dir, projectConfigFileName, "cli: claude\nhost_git_config: false\n")

		c, err := Load(dir, Config{})
		require.NoError(t, err)
		assert.Equal(t, new(false), c.HostGitConfig)
	})
}

func TestLoad_EmptyListKeepsLowerLayers(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectConfigFileName), []byte("allowlist: []\n"), ioutil.File))

	c, err := Load(dir, Config{})
	require.NoError(t, err)
	assert.Equal(t, []string{DefaultsToken}, c.Allowlist) // lists append: [] adds nothing, the seed's token stays
}

func TestLoad_LayersFiles(t *testing.T) {
	dir := t.TempDir()
	mkDirs(t, dir, tmpfsDefaults...)
	mkDirs(t, dir, "dist", "build", "cache") // the layers' own mask dirs must exist to survive
	bodies := map[string]string{             // one entry per configFiles term
		"user":    "cli: grok\ntmpfs_masks:\n  - ccbox-defaults\n  - dist\nenv:\n  FOO: user\n  BAR: user\nallowlist:\n  - user.example.dev\n",
		"project": "cli: claude\ntmpfs_masks:\n  - build\nenv:\n  FOO: project\n  BAZ: project\nallowlist:\n  - ccbox-defaults\nhost_git_config: true\n",
		"local":   "cli: codex\ntmpfs_masks:\n  - cache\nenv:\n  FOO: local\nallowlist:\n  - example.com\nhost_git_config: false\n",
	}
	for _, cf := range configFiles {
		writeConfig(t, dir, cf.name, bodies[cf.term])
	}

	c, err := Load(dir, Config{})
	require.NoError(t, err)
	assert.Equal(t, "codex", *c.CLIName)         // later layer wins
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
			assert.Equal(t, "grok", *c.CLIName)
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
			path := writeConfig(t, dir, cf.name, "tmpfs_masks: [")

			_, err := Load(dir, Config{})
			require.Error(t, err)
			assert.ErrorContains(t, err, path) // the error names the offending file
		})
	}
}

func TestLoad_RejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"unknown cli", "cli: emacs\n", []string{"CLIName", "is not one of [claude codex grok opencode]"}},
		{"absolute tmpfs_masks", "tmpfs_masks:\n  - /etc\n", []string{"TmpfsMasks", "Match"}},
		{"tmpfs_masks traversal", "tmpfs_masks:\n  - ../escape\n", []string{"TmpfsMasks", "Match"}},
		{"absolute volume_masks", "volume_masks:\n  - /var\n", []string{"VolumeMasks", "Match"}},
		{"absolute read_only_globs", "read_only_globs:\n  - /etc\n", []string{"ReadOnlyGlobs", "Match"}},
		{"read_only_globs traversal", "read_only_globs:\n  - dist/../x\n", []string{"ReadOnlyGlobs", "Match"}},
		{"read_only_globs bad glob char", "read_only_globs:\n  - \"dist/{a,b}\"\n", []string{"ReadOnlyGlobs", "Match"}},
		{"bad env key", "env:\n  bad-key: \"1\"\n", []string{"Env", "Match"}},
		{"empty env value", "env:\n  FOO: \"\"\n", []string{"Env", "Present"}},
		{"bad allow domain", "allowlist:\n  - \"https://x.dev\"\n", []string{"Allowlist", "Match"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, projectConfigFileName), []byte(tt.body), ioutil.File))

			_, err := Load(dir, Config{})
			require.Error(t, err)
			for _, want := range tt.want {
				assert.ErrorContains(t, err, want)
			}
		})
	}
}

func TestLoad_RejectsUnknownKeys(t *testing.T) {
	for _, cf := range configFiles {
		t.Run(cf.term, func(t *testing.T) {
			dir := t.TempDir()
			if cf.term == "local" { // empty project file so the error attributes to local
				writeConfig(t, dir, projectConfigFileName, "")
			}
			writeConfig(t, dir, cf.name, "cli: claude\nbogus: true\n")

			_, err := Load(dir, Config{})
			require.ErrorContains(t, err, "field bogus not found")
			assert.ErrorContains(t, err, cf.name) // the error names the offending file
		})
	}
}

func TestLoad_FlagsOverrideFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectConfigFileName), []byte("cli: claude\n"), ioutil.File))
	require.NoError(t, os.WriteFile(filepath.Join(dir, localConfigFileName), []byte("cli: codex\n"), ioutil.File))

	c, err := Load(dir, Config{CLIName: new("grok")})
	require.NoError(t, err)
	assert.Equal(t, "grok", *c.CLIName) // a set flag wins over both files

	c, err = Load(dir, Config{})
	require.NoError(t, err)
	assert.Equal(t, "codex", *c.CLIName) // an unset flag keeps the files' layering

	_, err = Load(dir, Config{CLIName: new("emacs")})
	assert.ErrorContains(t, err, "is not one of") // the flag value validates like a file's
}
