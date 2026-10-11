// The all-runtimes render test requires runtime, which imports mise — a circular
// dep, so it rides the external test package.
package mise_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/mise"
	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/prov"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

// TestRenderConfig_AllRuntimes pins the seed a fresh host gets: every runtime's
// real tools riding the render
func TestRenderConfig_AllRuntimes(t *testing.T) {
	all := prov.Resolved{Runtimes: runtime.All()}
	assert.Equal(t, golden(t, "TestRenderConfig_AllRuntimes.toml"), seedRender(t, all.MiseTools()))
}

// seedRender seeds the user-level config with tools, returning the seeded body
func seedRender(t *testing.T, tools []mise.Tool) string {
	t.Helper()
	testutil.Home(t)
	require.NoError(t, os.MkdirAll(userdir.Dir(), osutil.Dir))
	require.NoError(t, mise.SeedConfig(mise.UserConfigPath(), tools))
	body, err := os.ReadFile(mise.UserConfigPath()) // #nosec G304 -- the test's own seeded path
	require.NoError(t, err)
	return string(body)
}

// golden reads a render's testdata fixture
func golden(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name)) // #nosec G304 -- the test's own fixture
	require.NoError(t, err)
	return string(body)
}
