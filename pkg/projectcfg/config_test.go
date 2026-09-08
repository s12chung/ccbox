package projectcfg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/s12chung/ccbox/pkg/util/deepcopy"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

func TestConfig_ProjectDir(t *testing.T) {
	dir := t.TempDir()
	useHome(t) // no user file
	writeConfig(t, dir, projectConfigFileName, "cli: claude\nhost_git_config: true\n")

	c, err := Load(dir, Config{})
	require.NoError(t, err)
	assert.Equal(t, dir, c.ProjectDir())
}

func TestConfig_mergeOverlays(t *testing.T) {
	c := Config{
		CLI:           new("claude"),
		TmpfsMasks:    []string{"dist"},
		VolumeMasks:   []string{"target"},
		ReadOnlyGlobs: []string{".env"},
		Env:           map[string]string{"FOO": "base", "BAR": "base"},
		Allowlist:     []string{"ccbox-defaults"},
		HostGitConfig: new(true),
	}
	other := Config{
		CLI:           new("codex"),
		TmpfsMasks:    []string{"build"},
		VolumeMasks:   []string{"cache"},
		ReadOnlyGlobs: []string{".envrc"},
		Env:           map[string]string{"FOO": "local", "BAZ": "local"},
		Allowlist:     []string{"example.com"},
		HostGitConfig: new(false),
	}

	c.merge(other)

	assert.Equal(t, deepcopy.Of(other), other) // merging never touches the later layer
	assert.Equal(t, Config{
		CLI:           new("codex"),
		HostGitConfig: new(false), // a set later layer wins
		TmpfsMasks:    []string{"dist", "build"},
		VolumeMasks:   []string{"target", "cache"},
		ReadOnlyGlobs: []string{".env", ".envrc"},
		Env:           map[string]string{"FOO": "local", "BAR": "base", "BAZ": "local"},
		Allowlist:     []string{"ccbox-defaults", "example.com"},
	}, c)
}

func TestConfig_ListsExpanded(t *testing.T) {
	tests := []struct {
		name                string
		projectDir          string
		body                string
		want                Config
		wantTmpfsExpanded   []string
		wantVolumesExpanded []string
		wantGlobsExpanded   []string
		wantPathsPresent    []string
	}{
		{
			"an all-nil config expands every list to nothing", t.TempDir(),
			"cli: claude\nhost_git_config: true\n",
			Config{
				CLI:           new("claude"),
				HostGitConfig: new(true),
			},
			nil, nil, nil, nil,
		},
		{
			"the token expands in place, literals kept, repeats collapse", t.TempDir(),
			"cli: claude\nhost_git_config: true\n" +
				"tmpfs_masks:\n  - ccbox-defaults\n  - dist\n  - ccbox-defaults\n" +
				"read_only_globs:\n  - ccbox-defaults\n  - .env\n  - ccbox-defaults\n",
			Config{
				CLI:           new("claude"),
				HostGitConfig: new(true),
				TmpfsMasks:    []string{DefaultsToken, "dist", DefaultsToken},
				ReadOnlyGlobs: []string{DefaultsToken, ".env", DefaultsToken},
			},
			append(append([]string{}, tmpfsDefaults...), "dist"),
			nil,
			readOnlyDefaults,        // the literal ".env" collapses into the defaults' own entry
			[]string{".ccbox.yaml"}, // the token expands: the project's own config matches; .env is absent here
		},
		{
			"an explicit empty list stays empty like an unset one", t.TempDir(),
			"cli: claude\nhost_git_config: true\nvolume_masks: []\n",
			Config{CLI: new("claude"), HostGitConfig: new(true), VolumeMasks: []string{}},
			nil, nil, nil, nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useHome(t) // no user file: the project body stands alone
			writeConfig(t, tt.projectDir, projectConfigFileName, tt.body)
			tt.want.projectDir = tt.projectDir

			c, err := Load(tt.projectDir, Config{})
			require.NoError(t, err)
			assert.Equal(t, &tt.want, c)
			assert.Equal(t, tt.wantTmpfsExpanded, c.TmpfsMasksExpanded())
			assert.Equal(t, tt.wantVolumesExpanded, c.VolumeMasksExpanded())
			assert.Equal(t, tt.wantGlobsExpanded, c.ReadOnlyGlobsExpanded())
			assert.Equal(t, tt.wantPathsPresent, c.ReadOnlyPathsPresent())
		})
	}
}

func TestConfig_AccessorsCache(t *testing.T) {
	dir := t.TempDir()
	useHome(t) // no user file: no defaults token
	mkDirs(t, dir, "dist")
	writeConfig(t, dir, projectConfigFileName, "cli: claude\nhost_git_config: true\ntmpfs_masks:\n  - dist\n")

	c, err := Load(dir, Config{})
	require.NoError(t, err)

	expanded := c.TmpfsMasksExpanded()
	assert.Equal(t, []string{"dist"}, expanded)

	// the present-filter re-stats each call: the project dir's changing contents show through
	require.NoError(t, os.Remove(filepath.Join(dir, "dist")))
	assert.Nil(t, c.TmpfsMasksPresent())
	assert.Equal(t, expanded, c.TmpfsMasksExpanded()) // the cached expansion holds
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
			"tmpfs_masks:\n  - dist\nvolume_masks:\n  - target\n",
			nil, nil,
		},
		{
			"present stays, absent drops",
			[]string{"dist"},
			"tmpfs_masks:\n  - dist\n  - build\n",
			[]string{"dist"},
			nil, // the seed's defaults are all absent here
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			mkDirs(t, dir, tt.dirs...)
			writeConfig(t, dir, projectConfigFileName, tt.body)

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
			"tmpfs_masks:\n  - dist\nvolume_masks:\n  - node_modules\n  - target\n",
			[]string{"dist"},
			[]string{".venv", "target"},
		},
		{
			"all absent rejects every entry, token and own alike", absent,
			"tmpfs_masks:\n  - dist\n",
			append(append([]string{}, tmpfsDefaults...), "dist"),
			volumeDefaults,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.projectBody != "" {
				require.NoError(t, os.WriteFile(filepath.Join(tt.projectDir, projectConfigFileName), []byte(tt.projectBody), ioutil.File))
			}
			c, err := Load(tt.projectDir, Config{})
			require.NoError(t, err)
			assert.Equal(t, tt.wantTmpfsMasks, c.TmpfsMasksAbsent())
			assert.Equal(t, tt.wantVolumeMasks, c.VolumeMasksAbsent())
		})
	}
}

// readOnlyGlobsConfig loads a config over the project body, ready for the glob accessors
func readOnlyGlobsConfig(t *testing.T, dir string, globs ...string) *Config {
	t.Helper()
	body := "cli: claude\nhost_git_config: true\nread_only_globs:\n"
	var bodySb18 strings.Builder
	for _, g := range globs {
		bodySb18.WriteString("  - \"" + g + "\"\n") // quoted: a leading * is a YAML alias marker
	}
	body += bodySb18.String()
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectConfigFileName), []byte(body), ioutil.File))

	c, err := Load(dir, Config{})
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
				require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(projectDir, p)), ioutil.Dir))
				require.NoError(t, os.WriteFile(filepath.Join(projectDir, p), nil, ioutil.File))
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
	require.NoError(t, os.WriteFile(filepath.Join(dir, "build", "main.o"), nil, ioutil.File))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "certs", "server.pem"), nil, ioutil.File))
	body := "cli: claude\nhost_git_config: true\n" +
		"tmpfs_masks:\n  - build\n  - certs\n  - node_modules\n" +
		"read_only_globs:\n  - build\n  - \"build/**\"\n  - \"**/*.pem\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectConfigFileName), []byte(body), ioutil.File))

	c, err := Load(dir, Config{})
	require.NoError(t, err)

	// the expansion keeps the masked entries — the absent node_modules too — but their
	// matches drop: a mask is never re-mounted read-only, and certs/server.pem matches
	// **/*.pem only under one
	assert.Equal(t, []string{"build", "build/**", "**/*.pem"}, c.ReadOnlyGlobsExpanded())
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
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), nil, ioutil.File))
		assert.Equal(t, []string{".env"}, c.ReadOnlyPathsPresent())
	})

	t.Run("a run's removed path drops", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), nil, ioutil.File))
		c := readOnlyGlobsConfig(t, dir, ".env")
		require.NoError(t, os.Remove(filepath.Join(dir, ".env")))
		assert.Nil(t, c.ReadOnlyPathsPresent())
	})
}

func TestConfig_ReadOnlyGlobsExpanded(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), nil, ioutil.File))
	c := readOnlyGlobsConfig(t, dir, DefaultsToken, "typo-glob")

	// the token expands in place. The project's own .ccbox.yaml matches the defaults' first
	// entry, .env its third; the rest match nothing.
	assert.Equal(t, append(append([]string{}, readOnlyDefaults...), "typo-glob"), c.ReadOnlyGlobsExpanded())
	assert.Equal(t, []string{".ccbox.yaml", ".env"}, c.ReadOnlyPathsPresent())
}

func TestConfig_VolumeCleanupDirs(t *testing.T) {
	c := Config{VolumeMasks: []string{"node_modules", "target"}}
	assert.Equal(t, []string{"node_modules", ".venv", "vendor/bundle", "target"}, c.VolumeCleanupDirs())

	c = Config{VolumeMasks: []string{DefaultsToken, "target"}} // the token expands to the built-ins, deduped
	assert.Equal(t, []string{"node_modules", ".venv", "vendor/bundle", "target"}, c.VolumeCleanupDirs())
}

func TestConfig_MarshalYAMLResolves(t *testing.T) {
	dir := t.TempDir()
	// present stays resolved in, the absent drop: build, .venv, vendor/bundle, cache, .env
	mkDirs(t, dir, append(append([]string{}, tmpfsDefaults...), "dist", "node_modules", "target", "secrets")...)
	c := &Config{
		CLI:           new("claude"),
		HostGitConfig: new(true),
		TmpfsMasks:    []string{DefaultsToken, "dist", "build"},
		VolumeMasks:   []string{DefaultsToken, "target", "cache"},
		ReadOnlyGlobs: []string{DefaultsToken, ".env"},
		Env:           map[string]string{"FOO": "bar"},
		Allowlist:     []string{"user.example.dev", DefaultsToken, "example.com"},
		projectDir:    dir,
	}

	out, err := yaml.Marshal(c)
	require.NoError(t, err)

	// round-trip the marshal back: masks resolved (token expanded in place, present
	// filtered), globs resolved to their matches, allowlist resolved, the rest passed through
	var got Config
	require.NoError(t, yaml.Unmarshal(out, &got))
	want := Config{
		CLI:           new("claude"),
		HostGitConfig: new(true),
		TmpfsMasks:    append(append([]string{}, tmpfsDefaults...), "dist"),
		VolumeMasks:   []string{"node_modules", "target"},
		ReadOnlyGlobs: []string{"secrets"},
		Env:           map[string]string{"FOO": "bar"},
		Allowlist:     append(append([]string{"user.example.dev"}, AllowDefaults()...), "example.com"),
	}
	assert.Equal(t, want, got)
}

// TestConfig_renderTmpl pins the render output to the committed testdata fixtures,
// which `make lint` yq-checks as YAML. Regenerate with:
// `UPDATE_FIXTURES=1 go test ./pkg/projectcfg/ -run TestConfig_renderTmpl`
func TestConfig_renderTmpl(t *testing.T) {
	for _, tt := range []struct {
		name string // fixture name: testdata/<name>.ccbox.yaml
		c    Config
	}{
		{"init", Config{}},                       // `ccbox config init`'s fill-in scaffold
		{"user-seed", *userSeedConfig("claude")}, // the safe-seeded user-level config
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.c.renderTmpl()
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
