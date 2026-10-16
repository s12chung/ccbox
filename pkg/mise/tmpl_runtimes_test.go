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
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

// TestRenderConfig_AllRuntimes pins the seed a fresh host gets: every runtime's
// real tools and apt libs riding the render
func TestRenderConfig_AllRuntimes(t *testing.T) {
	all := runtime.MiseRuntimes{Runtimes: runtime.All()}
	configSet := mise.ConfigSet{Tools: all.Tools(), Packages: all.Packages()}
	assert.Equal(t, golden(t, "TestRenderConfig_AllRuntimes.toml"), seedRender(t, configSet))
}

// seedRender seeds the user-level config with the config set, returning the
// seeded body
func seedRender(t *testing.T, configSet mise.ConfigSet) string {
	t.Helper()
	testutil.Home(t)
	require.NoError(t, os.MkdirAll(userdir.Dir(), osutil.Dir))
	require.NoError(t, mise.SeedConfig(mise.UserConfigPath(), configSet))
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
