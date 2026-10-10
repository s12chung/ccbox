package dockerfile

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegions pins each generated region's render.
func TestRegions(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"runtimeLibs", "        libatomic1 libssl3t64 libyaml-0-2 zlib1g libffi8 libreadline8t64 libgmp10 libzstd1 \\"},
		{"buildDeps", "libssl-dev libyaml-dev zlib1g-dev libffi-dev libreadline-dev libgmp-dev"},
		{"PATHDirs", "/home/ccbox/go/bin:/home/ccbox/.npm-global/bin:/home/ccbox/.gem/bin"},
		{"envVars", "GOBIN=/home/ccbox/go/bin NPM_CONFIG_PREFIX=/home/ccbox/.npm-global NPM_CONFIG_UPDATE_NOTIFIER=false GEM_HOME=/home/ccbox/.gem"},
		{"cacheDirs", "/home/ccbox/go /home/ccbox/.npm /home/ccbox/.npm-global /home/ccbox/.gem"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generate, ok := regions[tt.name].(func() string)
			require.True(t, ok)
			assert.Equal(t, tt.want, generate())
		})
	}
}

func TestRender(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("testdata", "TestRender.Dockerfile")) // #nosec G304 -- the test's own fixture
	require.NoError(t, err)

	composed, err := render()
	require.NoError(t, err)
	assert.Equal(t, string(golden), string(composed))
}

func TestPatchFS(t *testing.T) {
	fsys := fstest.MapFS{"docker/desktop.sh": {Data: []byte("echo")}}
	merged, err := PatchFS(fsys)
	require.NoError(t, err)

	patched, err := fs.ReadFile(merged, "Dockerfile")
	require.NoError(t, err)
	assert.Contains(t, string(patched), "RUN apt-get update")
	assert.Contains(t, string(patched), "ENV PATH=", "the render's regions are in")

	_, err = fs.ReadFile(merged, "docker/desktop.sh")
	require.NoError(t, err, "the rest of the fs rides untouched")
}
