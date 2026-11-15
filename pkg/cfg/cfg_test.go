package cfg

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/s12chung/firm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/models/cli"
	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/util/deepcopy"
	"github.com/s12chung/ccbox/pkg/util/mergeempty"
	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/seed"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

// mkDirs creates the given project-relative dirs under dir, so presence-sensitive
// defaults pick them up.
func mkDirs(t *testing.T, dir string, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		// #nosec G703 -- the test's own paths: a temp project dir or TestMain's temp home
		require.NoError(t, os.MkdirAll(filepath.Join(dir, d), osutil.Dir))
	}
}

// useHome creates the user config's dir in a fresh temp home (testutil.Home).
// Returns the user config's path.
func useHome(t *testing.T) string {
	t.Helper()
	home := testutil.Home(t)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".ccbox", "config"), osutil.Dir))
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
	require.NoError(t, os.WriteFile(path, []byte(body), osutil.File))
	return path
}

func loadConfigYAML(t *testing.T, projectDir string) string {
	t.Helper()
	c, err := Load(projectDir, Flags{})
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
	require.NoError(t, os.WriteFile(dst, body, osutil.File))
}

// configFiles lists the config files by level term and file name, in load order
// (lowest precedence first)
var configFiles = []struct {
	term string // user, project, projectLocal
	name string
}{
	{"user", userConfigFileName},
	{"project", projectConfigFileName},
	{"projectLocal", projectLocalConfigFileName},
}

// TestMain points home at a temp tree and seeds the user config — prod parity: the
// user seed exists before Load runs — so tests don't read the developer's real
// user-level config; useHome overrides per-test.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "cfg")
	must.Do(err)
	must.Do(os.Setenv("HOME", dir))
	must.Do(SeedConfig(UserConfigFile(), "claude"))

	cli.Load()

	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func TestSeedConfig(t *testing.T) {
	dir := t.TempDir()
	mkDirs(t, dir, tmpfsDefaults...)
	mkDirs(t, dir, volumeDefaults...)
	useHome(t)

	require.NoError(t, SeedConfig(UserConfigFile(), "claude"))

	// the seed is the defaults' carrier
	want := Config{
		CLIName:       "claude",
		ForwardEnv:    []string{"TERM", "COLORTERM"},
		TmpfsMasks:    []string{DefaultsAlias},
		VolumeMasks:   []string{DefaultsAlias},
		ReadOnlyGlobs: []string{DefaultsAlias},
		ReadOnlyBinds: map[string]string{GitConfigKey: firmrule.EnabledValue},
		Proxy: Proxy{
			Enabled:   true,
			Allowlist: []string{DefaultsAlias, runtime.AllRuntimesAlias, SetHarnessAlias},
		},

		projectDir:   dir,
		serveProxied: true, // the default ask on the proxy-enabled seed
	}

	t.Run("seed alone loads the default config", func(t *testing.T) {
		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.Equal(t, &want, c)
	})

	t.Run("an empty project file is unset like a missing one", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, projectConfigFileName), nil, osutil.File))

		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.Equal(t, &want, c)
	})
}

func TestSeedConfig_SkipsExisting(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, userConfigFileName, "cli: codex\n")

	require.NoError(t, SeedConfig(UserConfigFile(), "claude"))

	// #nosec G304 -- the package's own temp user file
	body, err := os.ReadFile(UserConfigFile())
	require.NoError(t, err)
	assert.Equal(t, "cli: codex\n", string(body)) // the user's own file is never touched
}

func TestSeedConfig_ChoosesCLI(t *testing.T) {
	useHome(t)

	require.NoError(t, SeedConfig(UserConfigFile(), "codex"))

	body, err := os.ReadFile(UserConfigFile()) // #nosec G304 -- the package's own temp user file
	require.NoError(t, err)
	assert.Contains(t, string(body), "cli: codex\n")
}

func TestLoadedPaths(t *testing.T) {
	tests := []struct {
		name  string
		files []string // config file names present on disk
	}{
		{"no files", nil},
		{"project only", []string{projectConfigFileName}},
		{"user and local", []string{userConfigFileName, projectLocalConfigFileName}},
		{"all three", []string{userConfigFileName, projectConfigFileName, projectLocalConfigFileName}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useHome(t) // no user file unless written below
			dir := t.TempDir()
			for _, name := range tt.files {
				writeConfig(t, dir, name, "cli: claude\n")
			}
			var wantLoaded, wantNotLoaded []string
			for _, name := range []string{userConfigFileName, projectConfigFileName, projectLocalConfigFileName} {
				path := filepath.Join(dir, name)
				if name == userConfigFileName {
					path = UserConfigFile()
				}
				if slices.Contains(tt.files, name) {
					wantLoaded = append(wantLoaded, path)
				} else {
					wantNotLoaded = append(wantNotLoaded, path)
				}
			}
			loaded, notLoaded := LoadedPaths(dir)
			assert.Equal(t, wantLoaded, loaded)
			assert.Equal(t, wantNotLoaded, notLoaded)
		})
	}

	t.Run("an empty file counts as loaded", func(t *testing.T) {
		useHome(t)
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, projectConfigFileName), nil, osutil.File))

		loaded, notLoaded := LoadedPaths(dir)
		assert.Equal(t, []string{filepath.Join(dir, projectConfigFileName)}, loaded)
		assert.Equal(t, []string{UserConfigFile(), filepath.Join(dir, projectLocalConfigFileName)}, notLoaded)
	})
}

func TestInit_WritesLoadableDefault(t *testing.T) {
	projectDir := t.TempDir()
	require.NoError(t, Init(projectDir))
	configPath := filepath.Join(projectDir, projectConfigFileName)

	cOut := loadConfigYAML(t, projectDir)

	freshDir := t.TempDir()
	copyFile(t, configPath, filepath.Join(freshDir, projectConfigFileName))
	freshOut := loadConfigYAML(t, freshDir)

	assert.Equal(t, freshOut, cOut)
}

func TestInit_RefusesExisting(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectConfigFileName), []byte("tmpfs_masks: []\n"), osutil.File))

	assert.ErrorIs(t, Init(dir), seed.ErrExists)
}

// TestConfig_ValidateErrors pins the input view's validation errors — the key paths
// a failed load shows the user
func TestConfig_ValidateErrors(t *testing.T) {
	aliases := allowlistAliases()
	quoted := make([]string, len(aliases))
	for i, a := range aliases {
		quoted[i] = strconv.Quote(a)
	}
	c := config{
		CLIName:       new("claude"),
		TmpfsMasks:    new([]string{"/etc"}),
		ReadOnlyGlobs: new([]string{"../escape"}),
		ReadOnlyBinds: new(map[string]string{GitConfigKey: "/home/ccbox/.config/git", "~/fonts": "mnt"}),
		Proxy:         &proxy{Allowlist: []string{"X.AI"}},
	}
	errMap := firm.ValidateAny(c)
	require.NotEmpty(t, errMap)
	// ShowValue appends the failing element, bindErr the failing mount, to each error
	assert.Equal(t, strings.Join([]string{
		`cfg.config.Proxy.Allowlist.[0].DomainOrAlias: Allowlist[0] is not a domain or one of ` +
			fmt.Sprintf("%v", quoted) + `: "X.AI"`,
		`cfg.config.ReadOnlyBinds.[gitconfig].Bind: ReadOnlyBinds[gitconfig] must be enabled: "/home/ccbox/.config/git"`,
		`cfg.config.ReadOnlyBinds.[~/fonts].Bind: ReadOnlyBinds[~/fonts] must be a container mount path (~/… or /…, no .. segment): "mnt"`,
		`cfg.config.ReadOnlyGlobs.[0].Match: ReadOnlyGlobs[0] does not match ` +
			`^[A-Za-z0-9_.*-]*[A-Za-z0-9_*-][A-Za-z0-9_.*-]*(/[A-Za-z0-9_.*-]*[A-Za-z0-9_*-][A-Za-z0-9_.*-]*)*$: "../escape"`,
		`cfg.config.TmpfsMasks.[0].Match: TmpfsMasks[0] does not match ` +
			`^[.]?[^/~]*[^./~][^/~]*(/[^/~]*[^./~][^/~]*)*$: "/etc"`,
	}, ", "), errMap.Error())
}

// TestConfig_renderTmpl pins the render output to the committed testdata fixtures,
// which `make lint` yq-checks as YAML. Regenerate with:
// `UPDATE_FIXTURES=1 go test ./pkg/cfg/ -run TestConfig_renderTmpl`
func TestConfig_renderTmpl(t *testing.T) {
	for _, tt := range []struct {
		name string // fixture name: testdata/TestConfig_renderTmpl_<name>.ccbox.yaml
		c    config
	}{
		{"init", config{}},                       // `ccbox config init`'s fill-in scaffold
		{"user-seed", *userSeedConfig("claude")}, // the safe-seeded user-level config
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.c.renderTmpl()
			require.NoError(t, err)

			path := filepath.Join("testdata", "TestConfig_renderTmpl_"+tt.name+".ccbox.yaml")
			if os.Getenv("UPDATE_FIXTURES") != "" {
				require.NoError(t, os.MkdirAll(filepath.Dir(path), osutil.Dir))
				require.NoError(t, os.WriteFile(path, got, osutil.File))
			}

			// #nosec G304 -- the package's own fixture path
			want, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

// TestConfig_mergeOverlays pins the input view's fold: lists append, maps merge per key
func TestConfig_mergeOverlays(t *testing.T) {
	c := config{
		CLIName:       new("claude"),
		VNC:           &vnc{GUIAppName: "zcode", Enabled: new(true)},
		TmpfsMasks:    new([]string{"dist"}),
		VolumeMasks:   new([]string{"target"}),
		ReadOnlyGlobs: new([]string{".env"}),
		ReadOnlyBinds: new(map[string]string{GitConfigKey: firmrule.EnabledValue, "~/fonts": "/home/ccbox/fonts"}),
		ForwardEnv:    new([]string{"FOO", "BAR"}),
		Proxy: &proxy{
			Enabled:   new(true),
			Allowlist: []string{"ccbox-defaults"},
		},
	}
	other := config{
		CLIName:       new("codex"),
		VNC:           &vnc{},
		TmpfsMasks:    new([]string{"build"}),
		VolumeMasks:   new([]string{"cache"}),
		ReadOnlyGlobs: new([]string{".envrc"}),
		ReadOnlyBinds: new(map[string]string{"~/fonts": "/mnt/fonts", "~/certs": "/home/ccbox/certs"}),
		ForwardEnv:    new([]string{"FOO"}),
		Proxy:         &proxy{Allowlist: []string{"example.com"}},
	}

	mergeempty.Merge(&c, other)

	assert.Equal(t, deepcopy.Of(other), other) // merging never touches the later layer
	assert.Equal(t, config{
		CLIName: new("codex"),
		VNC: &vnc{ // the empty section merges per field: the later layer's unset fields keep the earlier's
			GUIAppName: "zcode",
			Enabled:    new(true),
		},
		TmpfsMasks:    new([]string{"dist", "build"}),
		VolumeMasks:   new([]string{"target", "cache"}),
		ReadOnlyGlobs: new([]string{".env", ".envrc"}),
		ReadOnlyBinds: new(map[string]string{ // maps merge per key, the later layer's entry wins
			GitConfigKey: firmrule.EnabledValue,
			"~/fonts":    "/mnt/fonts",
			"~/certs":    "/home/ccbox/certs",
		}),
		ForwardEnv: new([]string{"FOO", "BAR", "FOO"}), // lists append
		Proxy: &proxy{ // both layers set: per field; a nil later layer would keep the whole section
			Enabled:   new(true),                                 // a nil later field inherits, an explicit value wins
			Allowlist: []string{"ccbox-defaults", "example.com"}, // lists append
		},
	}, c)
}

func TestLoad_UnsetRequiredErrors(t *testing.T) {
	dir := t.TempDir()
	useHome(t) // no user file

	_, err := Load(dir, Flags{})
	require.Error(t, err)
	assert.ErrorContains(t, err, "CLIName.Nil: CLIName is nil")
}

func TestLoad_InvalidBindsErrors(t *testing.T) {
	for _, cf := range configFiles {
		t.Run(cf.term, func(t *testing.T) {
			dir := t.TempDir()
			if cf.term == "projectLocal" { // empty project file so the error attributes to local
				writeConfig(t, dir, projectConfigFileName, "")
			}
			writeConfig(t, dir, cf.name, "cli: claude\nread_only_binds:\n  fonts: enabled\n")

			_, err := Load(dir, Flags{})
			require.Error(t, err)
			assert.ErrorContains(t, err, "ReadOnlyBinds.[fonts]") // the error names the entry
		})
	}
}

func TestLoad_EmptyListKeepsLowerLayers(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectConfigFileName), []byte("proxy:\n  allowlist: []\n"), osutil.File))

	c, err := Load(dir, Flags{})
	require.NoError(t, err)
	// lists append: [] adds nothing, the seed's aliases stay
	assert.Equal(t, []string{DefaultsAlias, runtime.AllRuntimesAlias, SetHarnessAlias}, c.Proxy.Allowlist)
}

func TestLoad_LayersFiles(t *testing.T) {
	dir := t.TempDir()
	mkDirs(t, dir, tmpfsDefaults...)
	mkDirs(t, dir, "dist", "build", "cache") // the layers' own mask dirs must exist to survive
	bodies := map[string]string{             // one entry per configFiles term
		"user": "cli: opencode\ntmpfs_masks:\n  - ccbox-defaults\n  - dist\n" +
			"read_only_binds:\n  gitconfig: enabled\n  ~/fonts: /home/ccbox/fonts\n" +
			"forward_env:\n  - FOO\n  - BAR\nvnc:\n  gui_app: zcode\nproxy:\n  allowlist:\n    - user.example.dev\n",
		"project": "cli: claude\ntmpfs_masks:\n  - build\n" +
			"read_only_binds:\n  ~/fonts: /mnt/fonts\n  ~/certs: /home/ccbox/certs\n" +
			"forward_env:\n  - FOO\n  - BAZ\nproxy:\n  allowlist:\n    - ccbox-defaults\n",
		"projectLocal": "cli: codex\ntmpfs_masks:\n  - cache\n" +
			"read_only_binds:\n  ~/certs: /mnt/certs\n" +
			"forward_env:\n  - QUX\nproxy:\n  allowlist:\n    - example.com\n",
	}
	for _, cf := range configFiles {
		writeConfig(t, dir, cf.name, bodies[cf.term])
	}

	c, err := Load(dir, Flags{})
	require.NoError(t, err)
	assert.Equal(t, "codex", c.CLIName) // later layer wins

	// per-field merge: the local layer's unset section leaves the user's gui_app standing
	assert.Equal(t, VNC{GUIAppName: "zcode"}, c.VNC)

	// lists append raw, lowest layer first, the seed's aliases carried as-is
	assert.Equal(t, []string{DefaultsAlias, "dist", "build", "cache"}, c.TmpfsMasks)

	// forward_env appends, layer order
	assert.Equal(t, []string{"FOO", "BAR", "FOO", "BAZ", "QUX"}, c.ForwardEnv)

	// binds merge per key, the later layer's entry wins
	assert.Equal(t, map[string]string{
		"gitconfig": "enabled",
		"~/fonts":   "/mnt/fonts",
		"~/certs":   "/mnt/certs",
	}, c.ReadOnlyBinds)

	// the proxy section's lists append too, layer order
	assert.Equal(t, []string{"user.example.dev", DefaultsAlias, "example.com"}, c.Proxy.Allowlist)
}

func TestLoad_SingleFileOnly(t *testing.T) {
	for _, cf := range configFiles { // the other files are absent
		t.Run(cf.term, func(t *testing.T) {
			dir := t.TempDir()
			writeConfig(t, dir, cf.name, "cli: pi\nproxy:\n  allowlist:\n    - example.com\n")

			c, err := Load(dir, Flags{})
			require.NoError(t, err)
			assert.Equal(t, "pi", c.CLIName)
			if cf.term == "user" { // the user file replaces the seed wholesale
				assert.Equal(t, []string{"example.com"}, c.Proxy.Allowlist) // no alias → no defaults pulled in
			} else { // the user seed's aliases still underlie the project/local file
				assert.Equal(t, []string{DefaultsAlias, runtime.AllRuntimesAlias, SetHarnessAlias, "example.com"}, c.Proxy.Allowlist)
			}
		})
	}
}

func TestLoad_EmptyGuiAppNamesNoApp(t *testing.T) {
	// per-field merge: the project's enabled-only vnc overlays the seed's — gui_app unset on both sides
	dir := t.TempDir()
	writeConfig(t, dir, projectConfigFileName, "vnc:\n  enabled: true\n")

	c, err := Load(dir, Flags{})
	require.NoError(t, err)
	assert.Equal(t, VNC{Enabled: true}, c.VNC)
}

func TestLoad_VNC(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, userConfigFileName, "cli: claude\nvnc:\n  enabled: true\n")
	writeConfig(t, dir, projectConfigFileName, "vnc:\n  enabled: false\n")

	c, err := Load(dir, Flags{})
	require.NoError(t, err)
	assert.Equal(t, VNC{Enabled: false}, c.VNC) // the explicit false wins over the user layer's true
	assert.False(t, c.ServeVNC())
}

func TestLoad_Proxy(t *testing.T) {
	t.Run("a missing section loads as an empty one", func(t *testing.T) {
		useHome(t) // no user file: no section anywhere
		dir := t.TempDir()
		writeConfig(t, dir, projectConfigFileName, "cli: claude\n")

		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.Equal(t, Proxy{}, c.Proxy)
	})

	t.Run("the section's enabled loads as written", func(t *testing.T) {
		useHome(t) // no user file: the project's section stands alone
		dir := t.TempDir()
		writeConfig(t, dir, projectConfigFileName, "cli: claude\nproxy:\n  enabled: false\n")

		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.Equal(t, Proxy{Enabled: false}, c.Proxy)
	})

	t.Run("the section's enabled overrides the seed", func(t *testing.T) {
		dir := t.TempDir() // TestMain's seed is the user layer
		writeConfig(t, dir, projectConfigFileName, "proxy:\n  enabled: false\n")

		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.Equal(t, Proxy{
			Enabled:   false,
			Allowlist: []string{DefaultsAlias, runtime.AllRuntimesAlias, SetHarnessAlias}, // unset inherits
		}, c.Proxy)
	})
}

func TestLoad_InvalidErrors(t *testing.T) {
	for _, cf := range configFiles { // malformed yaml in any file errors
		t.Run(cf.term, func(t *testing.T) {
			dir := t.TempDir()
			path := writeConfig(t, dir, cf.name, "tmpfs_masks: [")

			_, err := Load(dir, Flags{})
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
		{"unknown cli", "cli: emacs\n", []string{"CLIName", "is not one of [\"claude\" \"codex\" \"opencode\" \"pi\"]"}},
		{"absolute tmpfs_masks", "tmpfs_masks:\n  - /etc\n", []string{"TmpfsMasks", "Match"}},
		{"tmpfs_masks traversal", "tmpfs_masks:\n  - ../escape\n", []string{"TmpfsMasks", "Match"}},
		{"absolute volume_masks", "volume_masks:\n  - /var\n", []string{"VolumeMasks", "Match"}},
		{"absolute read_only_globs", "read_only_globs:\n  - /etc\n", []string{"ReadOnlyGlobs", "Match"}},
		{"read_only_globs traversal", "read_only_globs:\n  - dist/../x\n", []string{"ReadOnlyGlobs", "Match"}},
		{"read_only_globs bad glob char", "read_only_globs:\n  - \"dist/{a,b}\"\n", []string{"ReadOnlyGlobs", "Match"}},
		{"bad forward_env name", "forward_env:\n  - bad-name\n", []string{"ForwardEnv", "Match"}},
		{"empty forward_env name", "forward_env:\n  - \"\"\n", []string{"ForwardEnv", "Match"}},
		{
			"unknown gui_app", "vnc:\n  gui_app: emacs\n",
			[]string{"GUIAppName", `is not one of ["zcode" ""]`},
		},
		{"bad allow domain", "proxy:\n  allowlist:\n    - \"https://x.dev\"\n", []string{"Allowlist", "DomainOrAlias"}},
		{"bare TLD allow domain", "proxy:\n  allowlist:\n    - com\n", []string{"Allowlist", "DomainOrAlias"}},
		{"allow domain with a newline", "proxy:\n  allowlist:\n    - \"foo.com\\n\"\n", []string{"Allowlist", "DomainOrAlias"}},
		{
			"unknown allowlist alias", "proxy:\n  allowlist:\n    - ccbox-bogus-alias\n",
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
			require.NoError(t, os.WriteFile(filepath.Join(dir, projectConfigFileName), []byte(tt.body), osutil.File))

			_, err := Load(dir, Flags{})
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
			if cf.term == "projectLocal" { // empty project file so the error attributes to local
				writeConfig(t, dir, projectConfigFileName, "")
			}
			writeConfig(t, dir, cf.name, "cli: claude\nbogus: true\n")

			_, err := Load(dir, Flags{})
			require.ErrorContains(t, err, "field bogus not found")
			assert.ErrorContains(t, err, cf.name) // the error names the offending file
		})
	}
}

func TestLoad_FlagsOverrideFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectConfigFileName), []byte("cli: claude\n"), osutil.File))
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectLocalConfigFileName), []byte("cli: codex\n"), osutil.File))

	c, err := Load(dir, Flags{CLI: new("pi")})
	require.NoError(t, err)
	assert.Equal(t, "pi", c.CLIName) // a set flag wins over both files

	c, err = Load(dir, Flags{})
	require.NoError(t, err)
	assert.Equal(t, "codex", c.CLIName) // an unset flag keeps the files' layering

	_, err = Load(dir, Flags{CLI: new("emacs")})
	assert.ErrorContains(t, err, "is not one of") // the flag value validates like a file's
}

func TestLoad_PortBindingsTCP(t *testing.T) {
	const portsYAML = "default_variant:\n  port_bindings:\n    tcp: 3000, 7200\n" // spaced csv: the loaded ports come out trimmed

	t.Run("the section binds the default ask's devbox, the other asks' do not", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, projectConfigFileName, "cli: claude\n"+portsYAML)

		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.Equal(t, []string{"3000", "7200"}, c.PortBindingsTCP())

		c, err = Load(dir, Flags{VNC: true})
		require.NoError(t, err)
		assert.Nil(t, c.PortBindingsTCP(), "a --vnc ask boots another devbox")

		c, err = Load(dir, Flags{NoProxy: true})
		require.NoError(t, err)
		assert.Nil(t, c.PortBindingsTCP(), "a --no-proxy ask boots another devbox")
	})

	t.Run("config-enabled vnc makes the no-flags ask the default one", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, projectConfigFileName, "cli: claude\nvnc:\n  enabled: true\n"+portsYAML)

		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.Equal(t, []string{"3000", "7200"}, c.PortBindingsTCP())
	})

	t.Run("the flag's list applies on any ask, over the files'", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, projectConfigFileName, "cli: claude\n"+portsYAML)

		c, err := Load(dir, Flags{PortBindingsTCP: new("8080"), VNC: true})
		require.NoError(t, err)
		assert.Equal(t, []string{"8080"}, c.PortBindingsTCP())

		c, err = Load(dir, Flags{PortBindingsTCP: new("8080"), NoProxy: true})
		require.NoError(t, err)
		assert.Equal(t, []string{"8080"}, c.PortBindingsTCP())
	})

	// the proxy command forces the egress wall on: on a proxy-disabled config, its ask boots
	// another devbox — a plain run's does not
	t.Run("the proxy command's forced ask on a proxy-disabled config boots another devbox", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, projectConfigFileName, "cli: claude\nproxy:\n  enabled: false\n"+portsYAML)

		c, err := Load(dir, Flags{ProxyCmd: true})
		require.NoError(t, err)
		assert.Nil(t, c.PortBindingsTCP())

		c, err = Load(dir, Flags{NoProxy: true})
		require.NoError(t, err)
		assert.Equal(t, []string{"3000", "7200"}, c.PortBindingsTCP())
	})

	t.Run("a later layer replaces the earlier layer's list wholesale", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, userConfigFileName, "cli: claude\nproxy:\n  enabled: true\n"+portsYAML)
		writeConfig(t, dir, projectConfigFileName, "cli: claude\ndefault_variant:\n  port_bindings:\n    tcp: 8080\n")

		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.Equal(t, []string{"8080"}, c.PortBindingsTCP())

		// the project layer unset: the user's list stands
		dir = t.TempDir()
		writeConfig(t, dir, userConfigFileName, "cli: claude\nproxy:\n  enabled: true\n"+portsYAML)
		c, err = Load(dir, Flags{})
		require.NoError(t, err)
		assert.Equal(t, []string{"3000", "7200"}, c.PortBindingsTCP())
	})

	t.Run("the flag value validates like a file's", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, projectConfigFileName, "cli: claude\n")
		for _, tcp := range []string{"70000", "3000,3000", "x"} {
			_, err := Load(dir, Flags{PortBindingsTCP: new(tcp)})
			require.ErrorContainsf(t, err, "PortList", "want %q invalid", tcp)
		}

		writeConfig(t, dir, projectLocalConfigFileName, "default_variant:\n  port_bindings:\n    tcp: 0\n")
		_, err := Load(dir, Flags{})
		require.Error(t, err)
		assert.ErrorContains(t, err, "PortList")
	})
}
