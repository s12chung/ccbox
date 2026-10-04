package projectcfg

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/pkg/cli"
	"github.com/s12chung/ccbox/pkg/guiapp"
	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/provider"
	"github.com/s12chung/ccbox/pkg/util/fsync"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/must"
)

// mkDirs creates the given project-relative dirs under dir, so presence-sensitive
// defaults pick them up.
func mkDirs(t *testing.T, dir string, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		// #nosec G703 -- the test's own paths: a temp project dir or TestMain's temp home
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
	c, err := Load(projectDir, Config{}, false)
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
	must.Do(err)
	must.Do(os.Setenv("HOME", dir))
	_, err = SeedUserConfig("claude")
	must.Do(err)

	cli.Load()

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
		VNC:           &VNC{Config: &pkginfo.VNCConfig{Resolution: pkginfo.DefaultResolution}},
		TmpfsMasks:    []string{DefaultsAlias},
		VolumeMasks:   []string{DefaultsAlias},
		ReadOnlyGlobs: []string{DefaultsAlias},
		ReadOnlyBinds: map[string]string{GitConfigKey: firmrule.EnabledValue},
		Allowlist:     []string{DefaultsAlias, SetHarnessAlias},

		projectDir: dir,
	}

	t.Run("seed alone loads the default config", func(t *testing.T) {
		c, err := Load(dir, Config{}, false)
		require.NoError(t, err)
		assert.Equal(t, &want, c)
	})

	t.Run("an empty project file is unset like a missing one", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, projectConfigFileName), nil, ioutil.File))

		c, err := Load(dir, Config{}, false)
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
	assert.ErrorIs(t, err, fsync.ErrExists)
}

func TestAllowlistDefaults_ExcludeProviders(t *testing.T) {
	// the defaults are tooling only: providers ride their own aliases —
	// ccbox-all-providers resolves every one of them
	providerDomains, ok := provider.DomainsFor(provider.AllProvidersAlias)
	require.True(t, ok)
	for _, d := range providerDomains {
		assert.NotContainsf(t, AllowlistDefaults(), d, "provider domain %s", d)
	}
}

func TestAllowlistDefaults_ExcludeGuiApp(t *testing.T) {
	// the GUI app's domains ride a vnc load's ccbox-set-harness expansion alone —
	// a headless run never downloads the app
	for _, d := range guiapp.App.AllowDomains {
		assert.NotContainsf(t, AllowlistDefaults(), d, "guiapp: %s", d)
	}
}

func TestLoad_UnsetRequiredErrors(t *testing.T) {
	dir := t.TempDir()
	useHome(t) // no user file

	_, err := Load(dir, Config{}, false)
	require.Error(t, err)
	assert.ErrorContains(t, err, "CLIName.Nil: CLIName is nil")
}

func TestLoad_InvalidBindsErrors(t *testing.T) {
	for _, cf := range configFiles {
		t.Run(cf.term, func(t *testing.T) {
			dir := t.TempDir()
			if cf.term == "local" { // empty project file so the error attributes to local
				writeConfig(t, dir, projectConfigFileName, "")
			}
			writeConfig(t, dir, cf.name, "cli: claude\nread_only_binds:\n  fonts: enabled\n")

			_, err := Load(dir, Config{}, false)
			require.Error(t, err)
			assert.ErrorContains(t, err, "ReadOnlyBinds.[fonts]") // the error names the entry
		})
	}
}

func TestLoad_EmptyListKeepsLowerLayers(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectConfigFileName), []byte("allowlist: []\n"), ioutil.File))

	c, err := Load(dir, Config{}, false)
	require.NoError(t, err)
	assert.Equal(t, []string{DefaultsAlias, SetHarnessAlias}, c.Allowlist) // lists append: [] adds nothing, the seed's aliases stay
}

func TestLoad_LayersFiles(t *testing.T) {
	dir := t.TempDir()
	mkDirs(t, dir, tmpfsDefaults...)
	mkDirs(t, dir, "dist", "build", "cache") // the layers' own mask dirs must exist to survive
	bodies := map[string]string{             // one entry per configFiles term
		"user": "cli: grok\ntmpfs_masks:\n  - ccbox-defaults\n  - dist\n" +
			"read_only_binds:\n  gitconfig: enabled\n  ~/fonts: /home/ccbox/fonts\n" +
			"env:\n  FOO: user\n  BAR: user\nvnc:\n  gui_app: zcode\n  config:\n    resolution: 1280x1024\nallowlist:\n  - user.example.dev\n",
		"project": "cli: claude\ntmpfs_masks:\n  - build\n" +
			"read_only_binds:\n  ~/fonts: /mnt/fonts\n  ~/certs: /home/ccbox/certs\n" +
			"env:\n  FOO: project\n  BAZ: project\nvnc:\n  config:\n    resolution: 1600x900\nallowlist:\n  - ccbox-defaults\n",
		"local": "cli: codex\ntmpfs_masks:\n  - cache\n" +
			"read_only_binds:\n  ~/certs: /mnt/certs\n" +
			"env:\n  FOO: local\nallowlist:\n  - example.com\n",
	}
	for _, cf := range configFiles {
		writeConfig(t, dir, cf.name, bodies[cf.term])
	}

	c, err := Load(dir, Config{}, false)
	require.NoError(t, err)
	assert.Equal(t, "codex", *c.CLIName) // later layer wins

	// per-field merge: the project's resolution wins, its unset gui_app keeps the user's;
	// the local layer's unset section leaves both standing
	require.NotNil(t, c.VNC)
	assert.Equal(t, &VNC{GUIAppName: "zcode", Config: &pkginfo.VNCConfig{Resolution: "1600x900"}}, c.VNC)

	// lists append raw, lowest layer first, the seed's aliases carried as-is
	assert.Equal(t, []string{DefaultsAlias, "dist", "build", "cache"}, c.TmpfsMasks)

	// env overlays, later wins
	assert.Equal(t, map[string]string{"FOO": "local", "BAR": "user", "BAZ": "project"}, c.Env)

	// binds merge per key, the later layer's entry wins
	assert.Equal(t, map[string]string{
		"gitconfig": "enabled",
		"~/fonts":   "/mnt/fonts",
		"~/certs":   "/mnt/certs",
	}, c.ReadOnlyBinds)

	assert.Equal(t, []string{"user.example.dev", DefaultsAlias, "example.com"}, c.Allowlist)
}

func TestLoad_SingleFileOnly(t *testing.T) {
	for _, cf := range configFiles { // the other files are absent
		t.Run(cf.term, func(t *testing.T) {
			dir := t.TempDir()
			writeConfig(t, dir, cf.name, "cli: grok\nallowlist:\n  - example.com\n")

			c, err := Load(dir, Config{}, false)
			require.NoError(t, err)
			assert.Equal(t, "grok", *c.CLIName)
			if cf.term == "user" { // the user file replaces the seed wholesale
				assert.Equal(t, []string{"example.com"}, c.Allowlist) // no alias → no defaults pulled in
			} else { // the user seed's aliases still underlie the project/local file
				assert.Equal(t, []string{DefaultsAlias, SetHarnessAlias, "example.com"}, c.Allowlist)
			}
		})
	}
}

func TestLoad_EmptyGuiAppNamesNoApp(t *testing.T) {
	// per-field merge: the project's config-only vnc overlays the seed's — gui_app unset on both sides
	dir := t.TempDir()
	writeConfig(t, dir, projectConfigFileName, "vnc:\n  config:\n    resolution: 1600x900\n")

	c, err := Load(dir, Config{}, false)
	require.NoError(t, err)
	assert.Equal(t, &VNC{Config: &pkginfo.VNCConfig{Resolution: "1600x900"}}, c.VNC)
}

func TestLoad_VNCWithoutConfig(t *testing.T) {
	// the unset config keeps the seed's — the desktop script's own resolution fallback
	dir := t.TempDir()
	writeConfig(t, dir, projectConfigFileName, "vnc:\n  gui_app: zcode\n")

	c, err := Load(dir, Config{}, false)
	require.NoError(t, err)
	assert.Equal(t, &VNC{GUIAppName: "zcode", Config: &pkginfo.VNCConfig{Resolution: pkginfo.DefaultResolution}}, c.VNC)
}

func TestLoad_EmptyResolutionInherits(t *testing.T) {
	// an explicit empty resolution is indistinguishable from an unset one — it inherits,
	// like every config field a later layer can't unset, only override
	dir := t.TempDir()
	writeConfig(t, dir, projectConfigFileName, "vnc:\n  config:\n    resolution:\n")

	c, err := Load(dir, Config{}, false)
	require.NoError(t, err)
	assert.Equal(t, &VNC{Config: &pkginfo.VNCConfig{Resolution: pkginfo.DefaultResolution}}, c.VNC)
}

func TestLoad_InvalidErrors(t *testing.T) {
	for _, cf := range configFiles { // malformed yaml in any file errors
		t.Run(cf.term, func(t *testing.T) {
			dir := t.TempDir()
			path := writeConfig(t, dir, cf.name, "tmpfs_masks: [")

			_, err := Load(dir, Config{}, false)
			require.Error(t, err)
			assert.ErrorContains(t, err, path) // the error names the offending file
		})
	}
}

// quotedAllowlistAliases renders allowlistAliases() like firm's OneOf error does: quoted
func quotedAllowlistAliases() []string {
	quoted := make([]string, 0, len(allowlistAliases()))
	for _, a := range allowlistAliases() {
		quoted = append(quoted, strconv.Quote(a))
	}
	return quoted
}

func TestLoad_RejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"unknown cli", "cli: emacs\n", []string{"CLIName", "is not one of [\"claude\" \"codex\" \"grok\" \"opencode\" \"pi\"]"}},
		{"absolute tmpfs_masks", "tmpfs_masks:\n  - /etc\n", []string{"TmpfsMasks", "Match"}},
		{"tmpfs_masks traversal", "tmpfs_masks:\n  - ../escape\n", []string{"TmpfsMasks", "Match"}},
		{"absolute volume_masks", "volume_masks:\n  - /var\n", []string{"VolumeMasks", "Match"}},
		{"absolute read_only_globs", "read_only_globs:\n  - /etc\n", []string{"ReadOnlyGlobs", "Match"}},
		{"read_only_globs traversal", "read_only_globs:\n  - dist/../x\n", []string{"ReadOnlyGlobs", "Match"}},
		{"read_only_globs bad glob char", "read_only_globs:\n  - \"dist/{a,b}\"\n", []string{"ReadOnlyGlobs", "Match"}},
		{"bad env key", "env:\n  bad-key: \"1\"\n", []string{"Env", "Match"}},
		{"empty env value", "env:\n  FOO: \"\"\n", []string{"Env", "Present"}},
		{"bad vnc resolution format", "vnc:\n  config:\n    resolution: 1600 by 900\n", []string{"VNC", "must be WxH"}},
		{"vnc resolution out of range", "vnc:\n  config:\n    resolution: 16385x900\n", []string{"VNC", "within 32..16384"}},
		{
			"unknown gui_app", "vnc:\n  gui_app: emacs\n  config:\n    resolution: 1600x900\n",
			[]string{"GUIAppName", `is not one of ["zcode" ""]`},
		},
		{"bad allow domain", "allowlist:\n  - \"https://x.dev\"\n", []string{"Allowlist", "DomainOrAlias"}},
		{
			"unknown allowlist alias", "allowlist:\n  - ccbox-bogus-alias\n",
			[]string{"Allowlist", "DomainOrAlias", "is not a domain or one of " + fmt.Sprintf("%v", quotedAllowlistAliases())},
		},
		{"bad bind key", "read_only_binds:\n  fonts: /mnt\n", []string{"ReadOnlyBinds.[fonts]", "must be gitconfig or a host path"}},
		{"bind key traversal", "read_only_binds:\n  ../escape: /mnt\n", []string{"ReadOnlyBinds.[../escape]", "must be gitconfig or a host path"}},
		{
			"bind mount not a path", "read_only_binds:\n  ~/fonts: mnt\n",
			[]string{"ReadOnlyBinds.[~/fonts]", "must be a container mount path"},
		},
		{
			"bind mount traversal", "read_only_binds:\n  ~/fonts: /mnt/../x\n",
			[]string{"ReadOnlyBinds.[~/fonts]", "must be a container mount path"},
		},
		{
			"enabled off the gitconfig key", "read_only_binds:\n  ~/fonts: enabled\n",
			[]string{"ReadOnlyBinds.[~/fonts]", "must be a container mount path"},
		},
		{
			"gitconfig pairs with enabled only", "read_only_binds:\n  gitconfig: /home/ccbox/.config/git\n",
			[]string{"ReadOnlyBinds.[gitconfig]", "must be enabled"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, projectConfigFileName), []byte(tt.body), ioutil.File))

			_, err := Load(dir, Config{}, false)
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

			_, err := Load(dir, Config{}, false)
			require.ErrorContains(t, err, "field bogus not found")
			assert.ErrorContains(t, err, cf.name) // the error names the offending file
		})
	}
}

func TestLoad_FlagsOverrideFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectConfigFileName), []byte("cli: claude\n"), ioutil.File))
	require.NoError(t, os.WriteFile(filepath.Join(dir, localConfigFileName), []byte("cli: codex\n"), ioutil.File))

	c, err := Load(dir, Config{CLIName: new("grok")}, false)
	require.NoError(t, err)
	assert.Equal(t, "grok", *c.CLIName) // a set flag wins over both files

	c, err = Load(dir, Config{}, false)
	require.NoError(t, err)
	assert.Equal(t, "codex", *c.CLIName) // an unset flag keeps the files' layering

	_, err = Load(dir, Config{CLIName: new("emacs")}, false)
	assert.ErrorContains(t, err, "is not one of") // the flag value validates like a file's
}
