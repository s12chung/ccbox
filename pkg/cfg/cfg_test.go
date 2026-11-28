package cfg

import (
	"fmt"
	"maps"
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
	"github.com/s12chung/ccbox/pkg/models/guiapp"
	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/util/deepcopy"
	"github.com/s12chung/ccbox/pkg/util/mergeempty"
	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/seed"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

var (
	testImages = Images{Base: "debian:trixie-20250113-slim", Mise: "ghcr.io/jdx/mise:2026.10.12"}
	imagesYAML = "images:\n  base: " + testImages.Base + "\n  mise: " + testImages.Mise + "\n"
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

// mkFiles creates empty files at the given project-relative paths under dir.
func mkFiles(t *testing.T, dir string, files ...string) {
	t.Helper()
	for _, f := range files {
		mkDirs(t, dir, filepath.Dir(f))
		// #nosec G703 -- the test's own path: a temp project dir
		require.NoError(t, os.WriteFile(filepath.Join(dir, f), nil, osutil.File))
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
	term string // user, project
	name string
}{
	{"user", userConfigFileName},
	{"project", projectConfigFileName},
}

// TestMain points home at a temp tree and seeds the user config — prod parity: the
// user seed exists before Load runs — so tests don't read the developer's real
// user-level config; useHome overrides per-test.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "cfg")
	must.Do(err)
	must.Do(os.Setenv("HOME", dir))
	must.Do(SeedConfig(UserConfigFile(), "pi", testImages))

	cli.Load()

	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func TestLoadedPaths(t *testing.T) {
	tests := []struct {
		name  string
		files []string // config file names present on disk
	}{
		{"no files", nil},
		{"project only", []string{projectConfigFileName}},
		{"user and project", []string{userConfigFileName, projectConfigFileName}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useHome(t) // no user file unless written below
			dir := t.TempDir()
			for _, name := range tt.files {
				writeConfig(t, dir, name, "cli: claude@latest\n"+imagesYAML)
			}
			var wantLoaded, wantNotLoaded []string
			for _, name := range []string{userConfigFileName, projectConfigFileName} {
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
		assert.Equal(t, []string{UserConfigFile()}, notLoaded)
	})
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
		CLIRef:        new("claude@latest"),
		TmpfsMasks:    new([]string{"/etc"}),
		ReadOnlyGlobs: new([]string{"../escape"}),
		ReadOnlyBinds: new(map[string]string{GitConfigKey: "/home/ccbox/.config/git", "~/fonts": "mnt"}),
		Proxy:         &proxy{Allowlist: []string{"X.AI"}},
	}
	errMap := firm.ValidateAny(c)
	require.NotEmpty(t, errMap)
	// ShowValue appends the failing element, bindErr the failing mount, to each error
	assert.Equal(t, strings.Join([]string{
		`cfg.config.Images.Nil: Images is nil`,
		`cfg.config.Proxy.Allowlist.[0].DomainOrAlias: Allowlist[0] is not a domain or one of ` +
			fmt.Sprintf("%v", quoted) + `: "X.AI"`,
		`cfg.config.ReadOnlyBinds.[gitconfig].Bind: ReadOnlyBinds[gitconfig] must be enabled: "/home/ccbox/.config/git"`,
		`cfg.config.ReadOnlyBinds.[~/fonts].Bind: ReadOnlyBinds[~/fonts] must be a container mount path (~/… or /…, no .. segment): "mnt"`,
		`cfg.config.ReadOnlyGlobs.[0].Match: ReadOnlyGlobs[0] does not match ` +
			`^[A-Za-z0-9_.*-]*[A-Za-z0-9_*-][A-Za-z0-9_.*-]*(/[A-Za-z0-9_.*-]*[A-Za-z0-9_*-][A-Za-z0-9_.*-]*)*$: "../escape"`,
		`cfg.config.TmpfsMasks.[0].Match: TmpfsMasks[0] does not match ` +
			`^[.]?[^/~]*[^./~][^/~]*(/[^/~]*[^./~][^/~]*)*$: "/etc"`,
	}, ", "), errMap.Error())

	t.Run("the images' refs must be full refs with a tag", func(t *testing.T) {
		c := config{
			CLIRef: new("claude@latest"),
			Images: &images{Base: "Debian", Mise: "ghcr.io/jdx/mise"},
		}

		errMap := firm.ValidateAny(c)

		require.NotEmpty(t, errMap)
		assert.Contains(t, errMap.Error(), `cfg.config.Images.Base.Match: Base does not match`)
		assert.Contains(t, errMap.Error(), `cfg.config.Images.Mise.Match: Mise does not match`)
	})
}

// TestConfig_renderTmpl pins the render output to the committed fixtures
// which `make lint` yq-checks as YAML. Regenerate with:
// `UPDATE_FIXTURES=1 go test ./pkg/cfg/ -run TestConfig_renderTmpl`
func TestConfig_renderTmpl(t *testing.T) {
	for _, tt := range []struct {
		name   string
		path   string
		c      config
		render func(*testing.T, *config) []byte
	}{
		// /docs is at the root dir
		{"docs", filepath.Join("..", "..", "docs", "config_commented.yaml"), *docsConfig(), renderDocs},
		{"user-seed", "testdata/TestConfig_renderTmpl_user-seed.ccbox.yaml", *userSeedConfig("pi", testImages), writtenConfig},
		{"init", "testdata/TestConfig_renderTmpl_init.ccbox.yaml", config{}, writtenConfig},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.render(t, &tt.c)

			if os.Getenv("UPDATE_FIXTURES") != "" {
				require.NoError(t, os.MkdirAll(filepath.Dir(tt.path), osutil.Dir))
				require.NoError(t, os.WriteFile(tt.path, got, osutil.File))
			}

			// #nosec G304 -- the package's own fixture path
			want, err := os.ReadFile(tt.path)
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

// docsConfig is the user seed with every section set
func docsConfig() *config {
	c := userSeedConfig("pi", testImages)
	c.DefaultVariant = &defaultVariant{PortBindings: &portBindings{TCP: "3000,8000,5173"}}
	c.HostGateway = &hostGateway{TCP: "5432"}
	c.VNC = &vnc{Enabled: new(true)}
	return c
}

func renderDocs(t *testing.T, c *config) []byte {
	t.Helper()
	body, err := c.renderTmpl()
	require.NoError(t, err)
	return []byte(strings.ReplaceAll(string(body), "##", "#"))
}

func writtenConfig(t *testing.T, c *config) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), projectConfigFileName)
	require.NoError(t, initConfig(path, *c))
	body, err := os.ReadFile(path) // #nosec G304 -- the test's own temp path
	require.NoError(t, err)
	return body
}

func TestRoGlobGroups_MatchDefaults(t *testing.T) {
	assert.ElementsMatch(t, roGlobDefaults, slices.Collect(maps.Keys(roGlobGroups)))
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

func TestSeedConfig(t *testing.T) {
	dir := t.TempDir()
	mkDirs(t, dir, tmpfsDefaults...)
	mkDirs(t, dir, volumeDefaults...)
	useHome(t)

	require.NoError(t, SeedConfig(UserConfigFile(), "pi", testImages))

	// the seed is the defaults' carrier
	want := Config{
		CLIRef:        "pi@latest",
		Images:        testImages,
		ForwardEnv:    []string{"TERM", "COLORTERM"},
		TmpfsMasks:    tmpfsDefaults,
		VolumeMasks:   volumeDefaults,
		ReadOnlyGlobs: roGlobDefaults,
		ReadOnlyBinds: map[string]string{GitConfigKey: firmrule.EnabledValue},
		Proxy: &Proxy{
			Allowlist: []string{DefaultsAlias, runtime.AllRuntimesAlias, SetHarnessAlias},
		},
		projectDir: dir,
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
	writeConfig(t, dir, userConfigFileName, "cli: codex@latest\n")

	require.NoError(t, SeedConfig(UserConfigFile(), "pi", testImages))

	// #nosec G304 -- the package's own temp user file
	body, err := os.ReadFile(UserConfigFile())
	require.NoError(t, err)
	assert.Equal(t, "cli: codex@latest\n", string(body)) // the user's own file is never touched
}

func TestSeedConfig_ChoosesCLI(t *testing.T) {
	useHome(t)

	require.NoError(t, SeedConfig(UserConfigFile(), "codex", testImages))

	body, err := os.ReadFile(UserConfigFile()) // #nosec G304 -- the package's own temp user file
	require.NoError(t, err)
	assert.Contains(t, string(body), "cli: codex@latest")
}

// TestConfig_mergeOverlays pins the input view's fold: lists append, maps merge per key
func TestConfig_mergeOverlays(t *testing.T) {
	c := config{
		CLIRef:        new("claude@latest"),
		VNC:           &vnc{GUIAppRef: new("zcode@latest"), Enabled: new(true)},
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
		CLIRef:        new("codex@0.9.2"), // a scalar: the later layer's value wins whole
		VNC:           &vnc{Enabled: new(false)},
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
		CLIRef: new("codex@0.9.2"),
		VNC: &vnc{ // the section merges per field: unset later fields keep the earlier's
			GUIAppRef: new("zcode@latest"),
			Enabled:   new(false), // an explicit later value wins — the --no-proxy fold's shape
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
			Enabled:   new(true),                                 // a nil later field inherits
			Allowlist: []string{"ccbox-defaults", "example.com"}, // lists append
		},
	}, c)
}

func TestLoad_UnsetRequiredErrors(t *testing.T) {
	dir := t.TempDir()
	useHome(t) // no user file

	_, err := Load(dir, Flags{})
	require.Error(t, err)
	assert.ErrorContains(t, err, "CLIRef.Nil: CLIRef is nil")
}

func TestLoad_InvalidBindsErrors(t *testing.T) {
	for _, cf := range configFiles {
		t.Run(cf.term, func(t *testing.T) {
			dir := t.TempDir()
			writeConfig(t, dir, cf.name, "cli: claude@latest\n"+imagesYAML+"read_only_binds:\n  fonts: enabled\n")

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
	mkDirs(t, dir, "dist", "build") // the layers' own mask dirs must exist to survive
	bodies := map[string]string{    // one entry per configFiles term
		"user": "cli: opencode@0.9.2\n" + imagesYAML + "tmpfs_masks:\n  - .idea\n  - dist\n" +
			"read_only_binds:\n  gitconfig: enabled\n  ~/fonts: /home/ccbox/fonts\n" +
			"forward_env:\n  - FOO\n  - BAR\nvnc:\n  enabled: true\n  gui_app: zcode@latest\nproxy:\n  enabled: true\n  allowlist:\n    - user.example.dev\n",
		"project": "cli: claude@latest\n" + imagesYAML + "tmpfs_masks:\n  - build\n" +
			"read_only_binds:\n  ~/fonts: /mnt/fonts\n  ~/certs: /home/ccbox/certs\n" +
			"forward_env:\n  - FOO\n  - BAZ\nproxy:\n  allowlist:\n    - ccbox-defaults\n",
	}
	for _, cf := range configFiles {
		writeConfig(t, dir, cf.name, bodies[cf.term])
	}

	c, err := Load(dir, Flags{})
	require.NoError(t, err)
	assert.Equal(t, "claude@latest", c.CLIRef) // later layer wins, whole: the user's version stays with the user's cli

	// per-field merge: the project layer's unset section leaves the user's gui_app standing
	assert.Equal(t, &VNC{GUIAppRef: new("zcode@latest")}, c.VNC)

	// lists append raw, lowest layer first
	assert.Equal(t, []string{".idea", "dist", "build"}, c.TmpfsMasks)

	// forward_env appends, layer order
	assert.Equal(t, []string{"FOO", "BAR", "FOO", "BAZ"}, c.ForwardEnv)

	// binds merge per key, the later layer's entry wins
	assert.Equal(t, map[string]string{
		"gitconfig": "enabled",
		"~/fonts":   "/mnt/fonts",
		"~/certs":   "/home/ccbox/certs",
	}, c.ReadOnlyBinds)

	// the proxy section's lists append too, layer order
	assert.Equal(t, []string{"user.example.dev", DefaultsAlias}, c.Proxy.Allowlist)
}

func TestLoad_SingleFileOnly(t *testing.T) {
	for _, cf := range configFiles { // the other files are absent
		t.Run(cf.term, func(t *testing.T) {
			dir := t.TempDir()
			writeConfig(t, dir, cf.name, "cli: pi@latest\n"+imagesYAML+"proxy:\n  enabled: true\n  allowlist:\n    - example.com\n")

			c, err := Load(dir, Flags{})
			require.NoError(t, err)
			assert.Equal(t, "pi", c.CLIName())
			if cf.term == "user" { // the user file replaces the seed wholesale
				assert.Equal(t, []string{"example.com"}, c.Proxy.Allowlist) // no alias → no defaults pulled in
			} else { // the user seed's aliases still underlie the project file
				assert.Equal(t, []string{DefaultsAlias, runtime.AllRuntimesAlias, SetHarnessAlias, "example.com"}, c.Proxy.Allowlist)
			}
		})
	}
}

func TestLoad_EmptyGuiAppNamesNoApp(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, projectConfigFileName, imagesYAML+"vnc:\n  enabled: true\n")

	c, err := Load(dir, Flags{})
	require.NoError(t, err)
	assert.Equal(t, &VNC{}, c.VNC)
	assert.False(t, c.HasGUIApp())
}

func TestLoad_VNC(t *testing.T) {
	t.Run("a section in any layer serves the desktop, the later layer's fields standing", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, userConfigFileName, "cli: claude@latest\n"+imagesYAML+"vnc:\n  enabled: true\n  gui_app: zcode@latest\n")
		writeConfig(t, dir, projectConfigFileName, imagesYAML+"vnc: {}\n")

		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.Equal(t, &VNC{GUIAppRef: new("zcode@latest")}, c.VNC)
	})

	t.Run("the --vnc flag adds a section with no app", func(t *testing.T) {
		useHome(t) // no user file
		dir := t.TempDir()
		writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+imagesYAML)

		c, err := Load(dir, Flags{VNC: true})
		require.NoError(t, err)
		assert.Equal(t, &VNC{}, c.VNC)
	})

	t.Run("a later layer's enabled: false opts the section out", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, userConfigFileName, "cli: claude@latest\n"+imagesYAML+"vnc:\n  enabled: true\n")
		writeConfig(t, dir, projectConfigFileName, imagesYAML+"vnc:\n  enabled: false\n")

		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.Nil(t, c.VNC)
	})
}

func TestLoad_GUIApp(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantName    string
		wantVersion *string
	}{
		{"a pinned gui_app", "vnc:\n  enabled: true\n  gui_app: zcode@1.2.3\n", "zcode", new("1.2.3")},
		{"a latest gui_app", "vnc:\n  enabled: true\n  gui_app: zcode@latest\n", "zcode", new("latest")},
		{"an empty gui_app", "vnc:\n  enabled: true\n  gui_app:\n", "", nil},
		{"no gui_app key", "vnc:\n  enabled: true\n", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+imagesYAML+tt.body)

			c, err := Load(dir, Flags{})
			require.NoError(t, err)

			if tt.wantName == "" {
				assert.False(t, c.HasGUIApp())
				return
			}
			assert.Equal(t, tt.wantName, c.VNC.GUIAppName())
			assert.Equal(t, tt.wantVersion, c.VNC.GUIAppVersion())
			assert.Equal(t, &guiapp.App, c.VNC.GUIApp())
		})
	}

	t.Run("a later layer's ref wins whole, version included", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, userConfigFileName, "cli: claude@latest\n"+imagesYAML+"vnc:\n  enabled: true\n  gui_app: zcode@1.2.3\n")
		writeConfig(t, dir, projectConfigFileName, imagesYAML+"vnc:\n  gui_app: zcode@latest\n")

		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.Equal(t, new("latest"), c.VNC.GUIAppVersion())
	})
}

func TestLoad_Proxy(t *testing.T) {
	t.Run("a missing section is no proxy", func(t *testing.T) {
		useHome(t) // no user file: no section anywhere
		dir := t.TempDir()
		writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+imagesYAML)

		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.Nil(t, c.Proxy)
	})

	t.Run("an older enabled: false opts the section out", func(t *testing.T) {
		useHome(t) // no user file: the project's section stands alone
		dir := t.TempDir()
		writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+imagesYAML+"proxy:\n  enabled: false\n")

		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.Nil(t, c.Proxy)
	})

	t.Run("the section inherits the seed's enabled and allowlist aliases", func(t *testing.T) {
		dir := t.TempDir() // TestMain's seed is the user layer
		writeConfig(t, dir, projectConfigFileName, imagesYAML+"proxy: {}\n")

		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.Equal(t, &Proxy{
			Allowlist: []string{DefaultsAlias, runtime.AllRuntimesAlias, SetHarnessAlias}, // unset inherits
		}, c.Proxy)
	})

	t.Run("the --no-proxy flag drops the section, the proxy command's re-add wins over it", func(t *testing.T) {
		dir := t.TempDir() // TestMain's seed is the user layer: the section stands

		c, err := Load(dir, Flags{NoProxy: true})
		require.NoError(t, err)
		assert.Nil(t, c.Proxy)

		c, err = Load(dir, Flags{NoProxy: true, ProxyCmd: true})
		require.NoError(t, err)
		assert.NotNil(t, c.Proxy)
	})

	t.Run("an absent enabled is off", func(t *testing.T) {
		useHome(t) // no user file: no seed's true to inherit
		dir := t.TempDir()
		writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+imagesYAML+"proxy:\n  allowlist:\n    - example.com\n")

		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.Nil(t, c.Proxy)
	})

	t.Run("the proxy command adds the section", func(t *testing.T) {
		useHome(t) // no user file: no section anywhere
		dir := t.TempDir()
		writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+imagesYAML)

		c, err := Load(dir, Flags{ProxyCmd: true})
		require.NoError(t, err)
		assert.Equal(t, &Proxy{}, c.Proxy)
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
		{"unknown cli", "cli: emacs@latest\n", []string{"CLIRef", `names no known cli: "emacs" is not one of ["antigravity" "claude" "opencode" "pi"]`}},
		{"version-less cli", "cli: claude\n", []string{"CLIRef", `is not a cli ref (name@latest or name@version): "claude"`}},
		{"range version", "cli: claude@^1.0\n", []string{"CLIRef", `is not at "latest" or an exact version (1.2.3, 2.0.0-beta.1): "^1.0"`}},
		{"dist-tag version", "cli: claude@next\n", []string{"CLIRef", `is not at "latest" or an exact version (1.2.3, 2.0.0-beta.1): "next"`}},
		{"absolute tmpfs_masks", "tmpfs_masks:\n  - /etc\n", []string{"TmpfsMasks", "Match"}},
		{"tmpfs_masks traversal", "tmpfs_masks:\n  - ../escape\n", []string{"TmpfsMasks", "Match"}},
		{"absolute volume_masks", "volume_masks:\n  - /var\n", []string{"VolumeMasks", "Match"}},
		{"absolute read_only_globs", "read_only_globs:\n  - /etc\n", []string{"ReadOnlyGlobs", "Match"}},
		{"read_only_globs traversal", "read_only_globs:\n  - dist/../x\n", []string{"ReadOnlyGlobs", "Match"}},
		{"read_only_globs bad glob char", "read_only_globs:\n  - \"dist/{a,b}\"\n", []string{"ReadOnlyGlobs", "Match"}},
		{"bad forward_env name", "forward_env:\n  - bad-name\n", []string{"ForwardEnv", "Match"}},
		{"empty forward_env name", "forward_env:\n  - \"\"\n", []string{"ForwardEnv", "Match"}},
		{
			"unknown gui_app", "vnc:\n  gui_app: emacs@latest\n",
			[]string{"GUIAppRef", `names no known gui app: "emacs" is not one of ["zcode"]`},
		},
		{"version-less gui_app", "vnc:\n  gui_app: zcode\n", []string{"GUIAppRef", `is not a gui app ref (name@latest or name@version): "zcode"`}},
		{
			"range gui_app version", "vnc:\n  gui_app: zcode@^1.0\n",
			[]string{"GUIAppRef", `is not at "latest" or an exact version (1.2.3, 2.0.0-beta.1): "^1.0"`},
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
			writeConfig(t, dir, cf.name, "cli: claude@latest\n"+"bogus: true\n")

			_, err := Load(dir, Flags{})
			require.ErrorContains(t, err, "field bogus not found")
			assert.ErrorContains(t, err, cf.name) // the error names the offending file
		})
	}
}

func TestLoad_FlagsOverrideFiles(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, userConfigFileName, "cli: pi@latest\n"+imagesYAML)
	writeConfig(t, dir, projectConfigFileName, "cli: claude@0.9.2\n"+imagesYAML)

	tests := []struct {
		name string
		cli  *string
		want string
	}{
		{"a versioned name stands", new("pi@1.2.3"), "pi@1.2.3"},
		{"a bare name is latest, whatever the files pin", new("claude"), "claude@latest"},
		{"an unset flag keeps the files' layering", nil, "claude@0.9.2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := Load(dir, Flags{CLI: tt.cli})
			require.NoError(t, err)
			assert.Equal(t, tt.want, c.CLIRef)
		})
	}

	t.Run("an unknown cli validates like a file's", func(t *testing.T) {
		_, err := Load(dir, Flags{CLI: new("emacs")})
		assert.ErrorContains(t, err, "names no known cli") // never reaching CLI()'s panic
	})
}

func TestLoad_PortBindingsTCP(t *testing.T) {
	const portsYAML = "default_variant:\n  port_bindings:\n    tcp: 3000, 7200\n" // spaced csv: the loaded ports come out trimmed

	t.Run("the section binds the config's own ask's devbox, the other asks' do not", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+portsYAML)

		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.Equal(t, []string{"3000", "7200"}, c.DefaultVariant.PortBindings.TCP)

		c, err = Load(dir, Flags{VNC: true})
		require.NoError(t, err)
		assert.Nil(t, c.DefaultVariant, "a --vnc ask boots another devbox")

		c, err = Load(dir, Flags{NoProxy: true})
		require.NoError(t, err)
		assert.Nil(t, c.DefaultVariant, "a --no-proxy ask boots another devbox")
	})

	t.Run("config-present vnc makes the no-flags ask the config's own", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+"vnc:\n  enabled: true\n"+portsYAML)

		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.Equal(t, []string{"3000", "7200"}, c.DefaultVariant.PortBindings.TCP)
	})

	t.Run("the flag's list applies on any ask, over the files'", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+portsYAML)

		c, err := Load(dir, Flags{PortBindingsTCP: new("8080"), VNC: true})
		require.NoError(t, err)
		assert.Equal(t, []string{"8080"}, c.DefaultVariant.PortBindings.TCP)

		c, err = Load(dir, Flags{PortBindingsTCP: new("8080"), NoProxy: true})
		require.NoError(t, err)
		assert.Equal(t, []string{"8080"}, c.DefaultVariant.PortBindings.TCP)
	})

	// the proxy command forces the egress wall on: on a proxy-less config, its ask boots
	// another devbox — a plain run's does not
	t.Run("the proxy command's forced ask on a proxy-less config boots another devbox", func(t *testing.T) {
		useHome(t) // no user file: no section anywhere
		dir := t.TempDir()
		writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+imagesYAML+portsYAML)

		c, err := Load(dir, Flags{ProxyCmd: true})
		require.NoError(t, err)
		assert.Nil(t, c.DefaultVariant)

		c, err = Load(dir, Flags{NoProxy: true})
		require.NoError(t, err)
		assert.Equal(t, []string{"3000", "7200"}, c.DefaultVariant.PortBindings.TCP)
	})

	t.Run("a later layer replaces the earlier layer's list wholesale", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, userConfigFileName, "cli: claude@latest\n"+portsYAML)
		writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+imagesYAML+"default_variant:\n  port_bindings:\n    tcp: 8080\n")

		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.Equal(t, []string{"8080"}, c.DefaultVariant.PortBindings.TCP)

		// the project layer unset: the user's list stands
		dir = t.TempDir()
		writeConfig(t, dir, userConfigFileName, "cli: claude@latest\n"+imagesYAML+portsYAML)
		c, err = Load(dir, Flags{})
		require.NoError(t, err)
		assert.Equal(t, []string{"3000", "7200"}, c.DefaultVariant.PortBindings.TCP)
	})

	t.Run("the flag value validates like a file's", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+imagesYAML)
		for _, tcp := range []string{"70000", "3000,3000", "x"} {
			_, err := Load(dir, Flags{PortBindingsTCP: new(tcp)})
			require.ErrorContainsf(t, err, "PortList", "want %q invalid", tcp)
		}

		writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+"default_variant:\n  port_bindings:\n    tcp: 0\n")

		// the flag's valid value replaces the files' broken one pre-validation
		c, err := Load(dir, Flags{PortBindingsTCP: new("8080")})
		require.NoError(t, err)
		assert.Equal(t, []string{"8080"}, c.DefaultVariant.PortBindings.TCP)

		_, err = Load(dir, Flags{})
		require.Error(t, err)
		assert.ErrorContains(t, err, "PortList")
	})
}

func TestLoad_HostGatewayTCP(t *testing.T) {
	const portsYAML = "host_gateway:\n  tcp: 5432,6379\n"

	t.Run("the section applies to every ask", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+portsYAML)

		for _, flags := range []Flags{{}, {VNC: true}, {VNC: true, PortBindingsTCP: new("8080")}} {
			c, err := Load(dir, flags)
			require.NoError(t, err)
			assert.Equal(t, []string{"5432", "6379"}, c.HostGatewayTCP())
		}
	})

	t.Run("the flag's list overrides the files' on any ask", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+portsYAML)

		c, err := Load(dir, Flags{HostGatewayTCP: new("3306")})
		require.NoError(t, err)
		assert.Equal(t, []string{"3306"}, c.HostGatewayTCP())

		c, err = Load(dir, Flags{HostGatewayTCP: new("3306"), VNC: true})
		require.NoError(t, err)
		assert.Equal(t, []string{"3306"}, c.HostGatewayTCP())
	})

	t.Run("a later layer replaces the earlier layer's list wholesale", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, userConfigFileName, "cli: claude@latest\n"+imagesYAML+portsYAML)
		writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+imagesYAML+"host_gateway:\n  tcp: 3306\n")

		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.Equal(t, []string{"3306"}, c.HostGatewayTCP())

		// the project layer unset: the user's list stands
		dir = t.TempDir()
		writeConfig(t, dir, userConfigFileName, "cli: claude@latest\n"+imagesYAML+portsYAML)
		c, err = Load(dir, Flags{})
		require.NoError(t, err)
		assert.Equal(t, []string{"5432", "6379"}, c.HostGatewayTCP())
	})

	t.Run("the flag value validates like a file's", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+imagesYAML)
		for _, tcp := range []string{"70000", "5432,5432", "x"} {
			_, err := Load(dir, Flags{HostGatewayTCP: new(tcp)})
			require.ErrorContainsf(t, err, "PortList", "want %q invalid", tcp)
		}

		writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+"host_gateway:\n  tcp: 0\n")
		_, err := Load(dir, Flags{})
		require.Error(t, err)
		assert.ErrorContains(t, err, "PortList")
	})
}
