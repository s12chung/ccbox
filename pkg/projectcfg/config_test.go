package projectcfg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

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
