package dockerfile

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/prov"
	"github.com/s12chung/ccbox/pkg/util/slicex"
)

// allSelection resolves every runtime: the render matches the pre-spec image,
// byte for byte.
func allSelection(t *testing.T) prov.Resolved {
	t.Helper()
	provSpec := prov.Spec{Runtimes: slicex.Minus(runtime.Names(), []string{runtime.Node().Name})}
	return provSpec.Resolve()
}

// TestRegions pins each generated region's render.
func TestRegions(t *testing.T) {
	tests := []struct {
		name   string
		render func(regions) string
		want   string
	}{
		{"runtimeLibs", regions.RuntimeLibs, "        libatomic1 libssl3t64 libyaml-0-2 zlib1g libffi8 libreadline8t64 libgmp10 libzstd1 \\"},
		{"buildDeps", regions.BuildDeps, "libssl-dev libyaml-dev zlib1g-dev libffi-dev libreadline-dev libgmp-dev"},
		{"PATHDirs", regions.PATHDirs, "/home/ccbox/go/bin:/home/ccbox/.npm-global/bin:/home/ccbox/.gem/bin"},
		{
			"envVars", regions.EnvVars,
			"GOBIN=/home/ccbox/go/bin NPM_CONFIG_PREFIX=/home/ccbox/.npm-global NPM_CONFIG_UPDATE_NOTIFIER=false GEM_HOME=/home/ccbox/.gem",
		},
		{"cacheDirs", regions.CacheDirs, "/home/ccbox/go /home/ccbox/.npm /home/ccbox/.npm-global /home/ccbox/.gem"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.render(regions{allSelection(t)}))
		})
	}
}

// TestRegions_Selection pins the regions narrowing to a subset selection,
// extras riding the libs row.
func TestRegions_Selection(t *testing.T) {
	provSpec := prov.Spec{Runtimes: []string{"go", "node"}, AptPackages: []string{"htop", "jq"}}
	resolved := provSpec.Resolve()

	tests := []struct {
		name   string
		render func(regions) string
		want   string
	}{
		{"runtimeLibs", regions.RuntimeLibs, "        libatomic1 htop jq \\"},
		{"buildDeps", regions.BuildDeps, ""},
		{"PATHDirs", regions.PATHDirs, "/home/ccbox/go/bin:/home/ccbox/.npm-global/bin"},
		{"envVars", regions.EnvVars, "GOBIN=/home/ccbox/go/bin NPM_CONFIG_PREFIX=/home/ccbox/.npm-global NPM_CONFIG_UPDATE_NOTIFIER=false"},
		{"cacheDirs", regions.CacheDirs, "/home/ccbox/go /home/ccbox/.npm /home/ccbox/.npm-global"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.render(regions{resolved}))
		})
	}
}

func TestRender(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("testdata", "TestRender.Dockerfile")) // #nosec G304 -- the test's own fixture
	require.NoError(t, err)

	composed, err := render(allSelection(t))
	require.NoError(t, err)
	assert.Equal(t, string(golden), string(composed))
}

// TestRender_SubsetSelection pins a subset selection's render.
func TestRender_SubsetSelection(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("testdata", "TestRender_SubsetSelection.Dockerfile")) // #nosec G304 -- the test's own fixture
	require.NoError(t, err)

	provSpec := prov.Spec{Runtimes: []string{"go", "node"}, AptPackages: []string{"htop", "jq"}}
	composed, err := render(provSpec.Resolve())
	require.NoError(t, err)
	assert.Equal(t, string(golden), string(composed))
}

func TestPatchFS(t *testing.T) {
	fsys := fstest.MapFS{"docker/desktop.sh": {Data: []byte("echo")}}
	merged, err := PatchFS(fsys, allSelection(t))
	require.NoError(t, err)

	patched, err := fs.ReadFile(merged, "Dockerfile")
	require.NoError(t, err)
	assert.Contains(t, string(patched), "RUN apt-get update")
	assert.Contains(t, string(patched), "ENV PATH=", "the render's regions are in")

	_, err = fs.ReadFile(merged, "docker/desktop.sh")
	require.NoError(t, err, "the rest of the fs rides untouched")
}
