package cfg

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/models/cli"
	"github.com/s12chung/ccbox/pkg/models/guiapp"
	"github.com/s12chung/ccbox/pkg/models/provider"
	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

func TestConfig_ProjectDir(t *testing.T) {
	dir := t.TempDir()
	useHome(t) // no user file
	writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+imagesYAML)

	c, err := Load(dir, Flags{})
	require.NoError(t, err)
	assert.Equal(t, dir, c.ProjectDir())
}

func TestConfig_CLI(t *testing.T) {
	c := Config{CLIRef: "claude@0.9.2"}
	assert.Equal(t, "claude", c.CLIName())
	assert.Equal(t, "0.9.2", c.CLIVersion())
	assert.Equal(t, cli.MustFor("claude"), c.CLI())

	bare := Config{CLIRef: "emacs@latest"}
	assert.PanicsWithValue(t, `cli: unknown cli "emacs"`, func() { bare.CLI() })
}

func TestConfig_HostGatewayTCP(t *testing.T) {
	ports := Config{HostGateway: &HostGateway{TCP: []string{"5432", "6379"}}}
	tests := []struct {
		name string
		c    Config
		want []string
	}{
		{"the section applies on every ask", ports, []string{"5432", "6379"}},
		{"an unset section is none", Config{}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.c.HostGatewayTCP())
		})
	}
}

func TestConfig_ListsUnique(t *testing.T) {
	tests := []struct {
		name             string
		projectDir       string
		body             string
		want             Config
		wantPathsPresent []string
	}{
		{
			"an all-nil config keeps every list nothing", t.TempDir(),
			"cli: claude@latest\n" + imagesYAML,
			Config{CLIRef: "claude@latest", Images: testImages},
			nil,
		},
		{
			"repeats collapse, order kept", t.TempDir(),
			"cli: claude@latest\n" + imagesYAML +
				"tmpfs_masks:\n  - .idea\n  - dist\n  - .idea\n" +
				"read_only_globs:\n  - .env\n  - .ccbox.yaml\n  - .env\n",
			Config{
				CLIRef:        "claude@latest",
				Images:        testImages,
				TmpfsMasks:    []string{".idea", "dist"},
				ReadOnlyGlobs: []string{".env", ".ccbox.yaml"},
			},
			[]string{".ccbox.yaml"}, // the project's own config matches; .env is absent here
		},
		{
			"an explicit empty list loads like an unset one", t.TempDir(),
			"cli: claude@latest\n" + imagesYAML + "volume_masks: []\n",
			Config{CLIRef: "claude@latest", Images: testImages},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useHome(t) // no user file: the project body stands alone
			writeConfig(t, tt.projectDir, projectConfigFileName, tt.body)
			tt.want.projectDir = tt.projectDir

			c, err := Load(tt.projectDir, Flags{})
			require.NoError(t, err)
			assert.Equal(t, &tt.want, c)
			assert.Equal(t, tt.wantPathsPresent, c.ReadOnlyPathsPresent())
		})
	}
}

func TestConfig_MasksPresent_Restats(t *testing.T) {
	dir := t.TempDir()
	useHome(t) // no user file: no seeded defaults
	mkDirs(t, dir, "dist")
	writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+imagesYAML+"tmpfs_masks:\n  - dist\n")

	c, err := Load(dir, Flags{})
	require.NoError(t, err)
	assert.Equal(t, []string{"dist"}, c.TmpfsMasksPresent())

	// the present-filter re-stats each call: the project dir's changing contents show through
	require.NoError(t, os.Remove(filepath.Join(dir, "dist")))
	assert.Nil(t, c.TmpfsMasksPresent())
}

// loadBindsConfig loads a config whose read_only_binds are the given raw map
func loadBindsConfig(t *testing.T, binds map[string]string) *Config {
	t.Helper()
	dir := t.TempDir()
	body, err := yaml.Marshal(config{
		CLIRef:        new("claude@latest"),
		Images:        new(images(testImages)),
		ReadOnlyBinds: new(binds),
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectConfigFileName), body, osutil.File))

	c, err := Load(dir, Flags{})
	require.NoError(t, err)
	return c
}

func TestConfig_ReadOnlyBinds(t *testing.T) {
	t.Run("expands the special entry and ~/ keys, keeps absolute keys", func(t *testing.T) {
		home := testutil.Home(t)

		c := loadBindsConfig(t, map[string]string{
			GitConfigKey:     firmrule.EnabledValue,
			"~/shared/fonts": "/home/ccbox/fonts",
			"/srv/ca":        "/home/ccbox/.local/share/ca",
		})

		assert.Equal(t, map[string]string{
			filepath.Join(home, ".config", "git"): GitConfigMount,
			filepath.Join(home, "shared/fonts"):   "/home/ccbox/fonts",
			"/srv/ca":                             "/home/ccbox/.local/share/ca",
		}, c.ReadOnlyBindsExpanded())
	})

	t.Run("gitconfig honors XDG_CONFIG_HOME", func(t *testing.T) {
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)

		c := loadBindsConfig(t, map[string]string{GitConfigKey: firmrule.EnabledValue})

		assert.Equal(t, map[string]string{filepath.Join(xdg, "git"): GitConfigMount}, c.ReadOnlyBindsExpanded())
	})

	t.Run("an unresolvable home panics like userdir", func(t *testing.T) {
		testutil.NoHome(t)

		c := &Config{ReadOnlyBinds: map[string]string{GitConfigKey: firmrule.EnabledValue}}

		assert.Panics(t, func() { c.ReadOnlyBindsExpanded() })
	})

	t.Run("present keeps the host dirs on disk, absent takes the rest", func(t *testing.T) {
		home := testutil.Home(t)
		require.NoError(t, os.MkdirAll(filepath.Join(home, ".config", "git"), osutil.Dir))

		c := loadBindsConfig(t, map[string]string{
			GitConfigKey: firmrule.EnabledValue,
			"~/fonts":    "/home/ccbox/fonts",
		})

		assert.Equal(t, map[string]string{
			filepath.Join(home, ".config", "git"): GitConfigMount,
		}, c.ReadOnlyBindsPresent())
		assert.Equal(t, map[string]string{
			filepath.Join(home, "fonts"): "/home/ccbox/fonts",
		}, c.ReadOnlyBindsAbsent())
	})

	t.Run("the expansion caches, the present-filter re-stats", func(t *testing.T) {
		home := testutil.Home(t)

		c := loadBindsConfig(t, map[string]string{GitConfigKey: firmrule.EnabledValue})
		expanded := c.ReadOnlyBindsExpanded()
		assert.Nil(t, c.ReadOnlyBindsPresent())

		require.NoError(t, os.MkdirAll(filepath.Join(home, ".config", "git"), osutil.Dir))
		assert.Equal(t, map[string]string{filepath.Join(home, ".config", "git"): GitConfigMount}, c.ReadOnlyBindsPresent())
		assert.Equal(t, expanded, c.ReadOnlyBindsExpanded()) // the cached expansion holds
	})
}

func TestConfig_MasksPresent(t *testing.T) {
	tests := []struct {
		name            string
		dirs            []string // present in the project dir
		files           []string // present in the project dir
		body            string
		wantTmpfsMasks  []string
		wantVolumeMasks []string
	}{
		{
			"all absent drops out", nil, nil,
			"tmpfs_masks:\n  - dist\nvolume_masks:\n  - target\n",
			nil, nil,
		},
		{
			"present stays, absent drops",
			[]string{"dist"},
			nil,
			"tmpfs_masks:\n  - dist\n  - build\n",
			[]string{"dist"},
			nil, // the seed's defaults are all absent here
		},
		{
			"a file counts under tmpfs_masks only",
			nil,
			[]string{"notes.txt"},
			"tmpfs_masks:\n  - notes.txt\nvolume_masks:\n  - notes.txt\n",
			[]string{"notes.txt"},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			mkDirs(t, dir, tt.dirs...)
			mkFiles(t, dir, tt.files...)
			writeConfig(t, dir, projectConfigFileName, tt.body)

			c, err := Load(dir, Flags{})
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
			"the seed's defaults absent from the project reject", present,
			"",
			nil,
			[]string{".venv", "target"},
		},
		{
			"the config's own absent dirs reject like the defaults", present,
			"tmpfs_masks:\n  - dist\nvolume_masks:\n  - node_modules\n  - target\n",
			[]string{"dist"},
			[]string{".venv", "target"},
		},
		{
			"all absent rejects every entry, seed's and own alike", absent,
			"tmpfs_masks:\n  - dist\n",
			append(append([]string{}, tmpfsDefaults...), "dist"),
			volumeDefaults,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.projectBody != "" {
				require.NoError(t, os.WriteFile(filepath.Join(tt.projectDir, projectConfigFileName), []byte(tt.projectBody), osutil.File))
			}
			c, err := Load(tt.projectDir, Flags{})
			require.NoError(t, err)
			assert.Equal(t, tt.wantTmpfsMasks, c.TmpfsMasksAbsent())
			assert.Equal(t, tt.wantVolumeMasks, c.VolumeMasksAbsent())
		})
	}
}

func TestConfig_TmpfsMasksByKind(t *testing.T) {
	// the present split by kind; the absent drop out of both
	dir := t.TempDir()
	mkDirs(t, dir, "dist")
	mkFiles(t, dir, "notes.txt")
	writeConfig(t, dir, projectConfigFileName, "tmpfs_masks:\n  - dist\n  - notes.txt\n  - build\n")

	c, err := Load(dir, Flags{})
	require.NoError(t, err)
	assert.Equal(t, []string{"dist"}, c.TmpfsMasksDirs())
	assert.Equal(t, []string{"notes.txt"}, c.TmpfsMasksFiles())
}

// readOnlyGlobsConfig loads a config over the project body, ready for the glob accessors
func readOnlyGlobsConfig(t *testing.T, dir string, globs ...string) *Config {
	t.Helper()
	body := "cli: claude@latest\n" + imagesYAML + "read_only_globs:\n"
	var bodySb18 strings.Builder
	for _, g := range globs {
		bodySb18.WriteString("  - \"" + g + "\"\n") // quoted: a leading * is a YAML alias marker
	}
	body += bodySb18.String()
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectConfigFileName), []byte(body), osutil.File))

	c, err := Load(dir, Flags{})
	require.NoError(t, err)
	return c
}

func TestConfig_ReadOnlyPathsPresent(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		globs []string
		want  []string
	}{
		{
			"literals match files and dirs",
			[]string{".env", "secrets/key.pem"},
			[]string{".env", "secrets"},
			[]string{".env", "secrets"},
		},
		{
			"absent paths don't match", nil,
			[]string{".env", "secrets"},
			nil,
		},
		{
			"**/ spans directories and matches the root",
			[]string{"server.pem", "certs/server.pem", "deep/certs/server.pem"},
			[]string{"**/*.pem"},
			[]string{"certs/server.pem", "deep/certs/server.pem", "server.pem"},
		},
		{
			"* spans a segment only",
			[]string{".env.local", "deep/.env.local", ".environment"},
			[]string{".env.*"},
			[]string{".env.local"},
		},
		{
			"a matched dir covers the matches under it",
			[]string{"secrets/api.key", "secrets/server.pem", "secrets/notes.txt", "server.pem"},
			[]string{"secrets", "**/*.pem", "**/*.key"},
			[]string{"secrets", "server.pem"},
		},
		{
			"a nested exact match under a matched dir is covered",
			[]string{"secrets/api.key", "secrets/notes.txt"},
			[]string{"secrets", "secrets/api.key"},
			[]string{"secrets"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useHome(t) // no user seed: the globs under test stand alone
			projectDir := t.TempDir()
			for _, p := range tt.files {
				require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(projectDir, p)), osutil.Dir))
				require.NoError(t, os.WriteFile(filepath.Join(projectDir, p), nil, osutil.File))
			}
			c := readOnlyGlobsConfig(t, projectDir, tt.globs...)

			assert.Equal(t, tt.want, c.ReadOnlyPathsPresent())
		})
	}
}

func TestConfig_ReadOnlyPathsPresent_MaskedWin(t *testing.T) {
	useHome(t) // no user seed: the globs under test stand alone
	dir := t.TempDir()
	mkDirs(t, dir, "build", "certs")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "build", "main.o"), nil, osutil.File))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "certs", "server.pem"), nil, osutil.File))
	body := "cli: claude@latest\n" + imagesYAML +
		"tmpfs_masks:\n  - build\n  - certs\n  - node_modules\n" +
		"read_only_globs:\n  - build\n  - \"build/**\"\n  - \"**/*.pem\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectConfigFileName), []byte(body), osutil.File))

	c, err := Load(dir, Flags{})
	require.NoError(t, err)

	// the list keeps the masked entries — the absent node_modules too — but their
	// matches drop: a mask is never re-mounted read-only, and certs/server.pem matches
	// **/*.pem only under one
	assert.Equal(t, []string{"build", "build/**", "**/*.pem"}, c.ReadOnlyGlobs)
	assert.Empty(t, c.ReadOnlyPathsPresent())
}

func TestConfig_ReadOnlyPathsPresent_Fresh(t *testing.T) {
	useHome(t) // no user seed: the globs under test stand alone

	t.Run("nothing matches yet — the before-run snapshot", func(t *testing.T) {
		c := readOnlyGlobsConfig(t, t.TempDir(), ".env")
		assert.Nil(t, c.ReadOnlyPathsPresent())
	})

	t.Run("a run's created path shows", func(t *testing.T) {
		dir := t.TempDir()
		c := readOnlyGlobsConfig(t, dir, ".env")
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), nil, osutil.File))
		assert.Equal(t, []string{".env"}, c.ReadOnlyPathsPresent())
	})

	t.Run("a run's removed path drops", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), nil, osutil.File))
		c := readOnlyGlobsConfig(t, dir, ".env")
		require.NoError(t, os.Remove(filepath.Join(dir, ".env")))
		assert.Nil(t, c.ReadOnlyPathsPresent())
	})
}

func TestConfig_ReadOnlyGlobs_Unique(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), nil, osutil.File))
	c := readOnlyGlobsConfig(t, dir, append(append([]string{}, roGlobDefaults...), "typo-glob", ".env")...)

	// repeats collapse in place. The project's own .ccbox.yaml matches the defaults' first
	// entry, .env its own; the rest match nothing.
	assert.Equal(t, append(append([]string{}, roGlobDefaults...), "typo-glob"), c.ReadOnlyGlobs)
	assert.Equal(t, []string{".ccbox.yaml", ".env"}, c.ReadOnlyPathsPresent())
}

func TestConfig_AllowlistExpanded(t *testing.T) {
	useHome(t) // no user seed: the allowlists under test stand alone
	dir := t.TempDir()

	// claude's own domains, its provider alias expanded; headless, so no GUI app domains
	harnessDomains := []string{"api.anthropic.com", "platform.claude.com", "mcp-proxy.anthropic.com", "statsig.anthropic.com", "sentry.io"}

	t.Run("aliases expand in place, literals kept, repeats collapse", func(t *testing.T) {
		writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+imagesYAML+
			"proxy:\n"+
			"  enabled: true\n"+
			"  allowlist:\n"+
			"    - ccbox-anthropic-provider\n"+
			"    - example.com\n"+
			"    - ccbox-defaults\n"+
			"    - ccbox-anthropic-provider\n"+
			"    - ccbox-set-harness\n")

		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		// the harness alias's api.anthropic.com collapses into the provider alias's —
		// one expansion, in place
		want := slices.Concat([]string{"api.anthropic.com", "example.com"}, AllowlistDefaults(),
			[]string{"platform.claude.com", "mcp-proxy.anthropic.com", "statsig.anthropic.com", "sentry.io"})
		assert.Equal(t, want, c.AllowlistExpanded())
	})

	t.Run("ccbox-all-providers resolves every provider's domains", func(t *testing.T) {
		writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+imagesYAML+"proxy:\n  enabled: true\n  allowlist:\n    - ccbox-all-providers\n")

		allProviders, ok := provider.Set().DomainsFor(provider.AllProvidersAlias)
		require.True(t, ok)
		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.Equal(t, allProviders, c.AllowlistExpanded())
	})

	t.Run("ccbox-all-runtimes resolves every runtime's domains", func(t *testing.T) {
		writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+imagesYAML+"proxy:\n  enabled: true\n  allowlist:\n    - ccbox-all-runtimes\n")

		allRuntimes, ok := runtime.Set().DomainsFor(runtime.AllRuntimesAlias)
		require.True(t, ok)
		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.Equal(t, allRuntimes, c.AllowlistExpanded())
	})

	t.Run("the GUI app's domains enter only through the harness alias on a vnc load", func(t *testing.T) {
		writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+imagesYAML+
			"vnc:\n  gui_app: zcode\nproxy:\n  enabled: true\n  allowlist:\n    - ccbox-set-harness\n")

		headlessConfig, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.Nil(t, headlessConfig.VNC)
		assert.Equal(t, harnessDomains, headlessConfig.AllowlistExpanded())

		// the flag turns the section on: its gui_app stands, its domains enter
		vncConfig, err := Load(dir, Flags{VNC: true})
		require.NoError(t, err)
		assert.Equal(t, &VNC{GUIAppName: "zcode"}, vncConfig.VNC)
		assert.Equal(t, slices.Concat(harnessDomains, guiapp.App.Setup.Proxied.AllowDomains), vncConfig.AllowlistExpanded())
	})

	t.Run("a present vnc section pulls the harness alias's GUI app domains without the flag", func(t *testing.T) {
		writeConfig(t, dir, projectConfigFileName, "cli: claude@latest\n"+imagesYAML+
			"vnc:\n  enabled: true\n  gui_app: zcode\nproxy:\n  enabled: true\n  allowlist:\n    - ccbox-set-harness\n")

		c, err := Load(dir, Flags{})
		require.NoError(t, err)
		assert.NotNil(t, c.VNC)
		assert.Equal(t, slices.Concat(harnessDomains, guiapp.App.Setup.Proxied.AllowDomains), c.AllowlistExpanded())
	})
}

func TestConfig_MarshalYAMLResolves(t *testing.T) {
	dir := t.TempDir()
	// present stays resolved in, the absent drop: build, .venv, vendor/bundle, cache, .env
	mkDirs(t, dir, append(append([]string{}, tmpfsDefaults...), "dist", "node_modules", "target", "secrets")...)
	mkFiles(t, dir, "notes.txt")
	t.Setenv("XDG_CONFIG_HOME", "") // the gitconfig bind resolves $HOME's XDG dir
	home := os.Getenv("HOME")       // TestMain's temp home
	mkDirs(t, home, ".config/git")
	c := &Config{
		CLIRef:         "claude@latest",
		Images:         testImages,
		DefaultVariant: &DefaultVariant{PortBindings: PortBindings{TCP: []string{"3000", "7200"}}},
		HostGateway:    &HostGateway{TCP: []string{"5432", "6379"}},
		VNC:            &VNC{GUIAppName: guiapp.App.Name},
		TmpfsMasks:     slices.Concat(tmpfsDefaults, []string{"dist", "build", "notes.txt"}),
		VolumeMasks:    append(append([]string{}, volumeDefaults...), "cache"),
		ReadOnlyGlobs:  roGlobDefaults,
		ReadOnlyBinds:  map[string]string{GitConfigKey: firmrule.EnabledValue},
		ForwardEnv:     []string{"FOO"},
		Proxy: &Proxy{
			Allowlist: []string{"user.example.dev", DefaultsAlias, "example.com"},
		},
		projectDir: dir,
	}

	out, err := yaml.Marshal(c)
	require.NoError(t, err)

	var got config
	require.NoError(t, yaml.Unmarshal(out, &got))
	want := config{
		CLIRef:         new("claude@latest"),
		Images:         new(images(testImages)),
		DefaultVariant: &defaultVariant{PortBindings: &portBindings{TCP: "3000,7200"}}, // the loaded ports render as csv
		HostGateway:    &hostGateway{TCP: "5432,6379"},                                 // the ask's ports render as csv
		VNC:            &vnc{Enabled: new(true), GUIAppName: guiapp.App.Name},          // the printed key pins the section on
		TmpfsMasks:     new(slices.Concat(tmpfsDefaults, []string{"dist", "notes.txt"})),
		VolumeMasks:    new([]string{"node_modules", "target"}),
		ReadOnlyGlobs:  new([]string{"secrets"}),
		ReadOnlyBinds: new(map[string]string{ // the gitconfig dir is present here: kept, resolved
			filepath.Join(home, ".config", "git"): GitConfigMount,
		}),
		ForwardEnv: new([]string{"FOO"}),
		Proxy: &proxy{ // the allowlist renders raw: the alias stays a token
			Enabled:   new(true),
			Allowlist: []string{"user.example.dev", DefaultsAlias, "example.com"},
		},
	}
	assert.Equal(t, want, got)
}

func TestConfig_RunConfigJSON(t *testing.T) {
	tests := []struct {
		name string
		c    Config
		want pkginfo.RunConfig
	}{
		{
			"every ask",
			Config{
				CLIRef:         "claude@0.9.2",
				DefaultVariant: &DefaultVariant{PortBindings: PortBindings{TCP: []string{"3000", "7200"}}},
				HostGateway:    &HostGateway{TCP: []string{"5432"}},
			},
			pkginfo.RunConfig{CLIVersion: "0.9.2", PortBindingsTCP: []string{"3000", "7200"}, HostGatewayTCP: []string{"5432"}},
		},
		{
			"no ports, latest",
			Config{CLIRef: "claude@latest"},
			pkginfo.RunConfig{CLIVersion: pkginfo.LatestVersion},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := tt.c.RunConfigJSON()
			require.NoError(t, err)

			// the container parses it back through firm validation — the host-built
			// config must stay valid there
			got, err := pkginfo.RunConfigFromJSON(body)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestVNC_InfoJSON(t *testing.T) {
	t.Run("with gui app", func(t *testing.T) {
		v := &VNC{GUIAppName: guiapp.App.Name}

		body, err := v.InfoJSON()
		require.NoError(t, err)

		// the container parses it back through firm validation — the host-built
		// config must stay valid there
		info, err := pkginfo.VNCInfoFromJSON(body)
		require.NoError(t, err)
		require.NotNil(t, info.GUIApp)
		assert.Equal(t, guiapp.App.GUIPkgInfo, *info.GUIApp)
	})

	t.Run("empty gui app", func(t *testing.T) {
		v := &VNC{}

		body, err := v.InfoJSON()
		require.NoError(t, err)
		assert.JSONEq(t, `{}`, body)
	})
}

func TestConfig_GUIApp(t *testing.T) {
	t.Run("the vnc section's gui app resolves", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, projectConfigFileName, "vnc:\n  gui_app: "+guiapp.App.Name+"\n")

		c, err := Load(dir, Flags{VNC: true})
		require.NoError(t, err)

		app := c.GUIApp()
		assert.Equal(t, &guiapp.App, app)
	})

	t.Run("no vnc section or an empty gui app name names none", func(t *testing.T) {
		for _, body := range []string{"", "vnc: {}\n"} {
			for _, vncFlag := range []bool{false, true} {
				dir := t.TempDir()
				if body != "" {
					writeConfig(t, dir, projectConfigFileName, body)
				}
				c, err := Load(dir, Flags{VNC: vncFlag})
				require.NoError(t, err)

				assert.Nil(t, c.GUIApp())
			}
		}
	})
}
