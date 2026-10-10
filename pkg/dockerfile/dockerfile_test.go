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
)

// TestRegions pin each generated statement to its testdata golden — the
// pre-composer Dockerfile's bytes. Where the data's runtime-sorted order reorders
// lines, the shift is functionally inert (ordering was hand-laid before).
func TestRegions(t *testing.T) {
	for name, generate := range regions {
		t.Run(name, func(t *testing.T) {
			golden, err := os.ReadFile(filepath.Join("testdata", "TestRegions_"+name)) // #nosec G304 -- the test's own fixture
			require.NoError(t, err)
			assert.Equal(t, string(golden), generate(runtime.All()))
		})
	}
}

func TestAptRegion_UnnotedRuntime(t *testing.T) {
	// a runtime without an aptRowComments entry renders the generic section comment
	runtimes := []runtime.Runtime{{Name: "go", AptPkgs: []string{"bison"}}}
	assert.Contains(t, aptRegion(runtimes), "# - Go runtime libs\n")
	assert.Contains(t, aptRegion(runtimes), "        bison \\\n")
}

// TestRender pins the embedded tmpl's render to its testdata golden: the tmpl and
// the composer must move together, and the golden is the composed whole `ccbox
// build` ships (make lint hadolints it).
func TestRender(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("testdata", "TestRender.Dockerfile")) // #nosec G304 -- the test's own fixture
	require.NoError(t, err)

	composed, err := render(tmplBody)
	require.NoError(t, err)
	assert.Equal(t, string(golden), string(composed))
}

func TestRender_TemplateErrors(t *testing.T) {
	tests := []struct {
		name     string
		template string
		wantErr  string
	}{
		{"unknown action", "{{bogus}}\n", `function "bogus" not defined`},
		{"unrendered region", "FROM base\n", "the template never renders"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := render([]byte(tt.template))
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
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
