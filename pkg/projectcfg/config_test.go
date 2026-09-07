package projectcfg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

func TestConfig_VolumeCleanupPaths(t *testing.T) {
	c := Config{VolumeMasks: []string{"node_modules", "target"}}
	assert.Equal(t, []string{"node_modules", ".venv", "vendor/bundle", "target"}, c.VolumeCleanupPaths())
}

// TestConfig_renderTmpl pins the render output to the committed testdata fixtures,
// which `make lint` yq-checks as YAML. Regenerate with:
// `UPDATE_FIXTURES=1 go test ./pkg/projectcfg/ -run TestConfig_renderTmpl`
func TestConfig_renderTmpl(t *testing.T) {
	for _, tt := range []struct {
		name string // fixture name: testdata/<name>.ccbox.yaml
		c    Config
	}{
		{"init", Config{}},                      // `ccbox config init`'s fill-in scaffold
		{"user-seed", userSeedConfig("claude")}, // the safe-seeded user-level config
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
