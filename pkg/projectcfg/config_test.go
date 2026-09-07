package projectcfg

import (
	"os"
	"path/filepath"
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
	writeConfig(t, dir, projectFileName, "cli: claude\nhost_git_config: true\n")

	c, err := Load(dir, Config{})
	require.NoError(t, err)
	assert.Equal(t, dir, c.ProjectDir())
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

func TestConfig_AccessorsCache(t *testing.T) {
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

func TestConfig_VolumeCleanupDirs(t *testing.T) {
	c := Config{VolumeMasks: []string{"node_modules", "target"}}
	assert.Equal(t, []string{"node_modules", ".venv", "vendor/bundle", "target"}, c.VolumeCleanupDirs())

	c = Config{VolumeMasks: []string{DefaultsToken, "target"}} // the token expands to the built-ins, deduped
	assert.Equal(t, []string{"node_modules", ".venv", "vendor/bundle", "target"}, c.VolumeCleanupDirs())
}

func TestConfig_MarshalYAMLResolves(t *testing.T) {
	dir := t.TempDir()
	// present stays resolved in, the absent drop: build, .venv, vendor/bundle, cache
	mkDirs(t, dir, append(append([]string{}, tmpfsDefaults...), "dist", "node_modules", "target")...)
	c := &Config{
		CLI:           new("claude"),
		HostGitConfig: new(true),
		TmpfsMasks:    []string{DefaultsToken, "dist", "build"},
		VolumeMasks:   []string{DefaultsToken, "target", "cache"},
		Env:           map[string]string{"FOO": "bar"},
		Allowlist:     []string{"user.example.dev", DefaultsToken, "example.com"},
		projectDir:    dir,
	}

	out, err := yaml.Marshal(c)
	require.NoError(t, err)

	// round-trip the marshal back: masks resolved (token expanded in place, present
	// filtered), allowlist resolved, the rest passed through
	var got Config
	require.NoError(t, yaml.Unmarshal(out, &got))
	want := Config{
		CLI:           new("claude"),
		HostGitConfig: new(true),
		TmpfsMasks:    append(append([]string{}, tmpfsDefaults...), "dist"),
		VolumeMasks:   []string{"node_modules", "target"},
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
