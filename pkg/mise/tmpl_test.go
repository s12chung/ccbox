// The render tests ride the external test package: runtime imports mise, so the
// package's own tests can't import it.
package mise_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/mise"
	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

// testTools is the fixture render's stand-in for runtime.AllMiseTools(): one plain
// binding and one with a postinstall — the two shapes row renders
var testTools = []mise.Tool{
	{Tool: "go", Version: "1.26"},
	{Tool: "python", Version: "3.13", PostInstall: "python -m pip install setuptools==82.0.1 wheel==0.47.0"},
}

// TestRenderConfig pins the fixture render to its golden
func TestRenderConfig(t *testing.T) {
	assert.Equal(t, golden(t, "TestRenderConfig.toml"), seedRender(t, testTools))
}

// TestRenderConfig_AllRuntimes pins the seed a fresh host gets: every runtime's
// real tools riding the render
func TestRenderConfig_AllRuntimes(t *testing.T) {
	assert.Equal(t, golden(t, "TestRenderConfig_AllRuntimes.toml"), seedRender(t, runtime.AllMiseTools()))
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
