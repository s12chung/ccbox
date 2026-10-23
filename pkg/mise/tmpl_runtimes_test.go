// The all-runtimes render test requires runtime, which imports mise — a circular
// dep, so it lives in the external test package.
package mise_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/mise"
	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

// TestRenderConfig_AllRuntimes pins the seed a fresh host gets: every runtime's
// real tools and apt libs in the render, the dev-workflow CLIs after them
func TestRenderConfig_AllRuntimes(t *testing.T) {
	all := runtime.MiseRuntimes(runtime.All())
	var tools []mise.Tool
	for _, rt := range all {
		tools = append(tools, rt.MiseTool)
		tools = append(tools, rt.MiseToolsOptional...)
	}
	rts := must.Get(mise.ResolvedTools(slices.Concat(tools, mise.DevTools()), mise.ToolState{Pins: map[string]string{
		"go": "1.26", "golangci-lint": "2.13.1", "node": "26.3.0", "npm:oxlint": "1.28.0", "npm:oxfmt": "0.18.0",
		"python": "3.13", "ruby": "3.4", "rust": "1.99",
		"pypi:setuptools": "82.0.1", "pypi:wheel": "0.47.0",
		"aqua:astral-sh/ruff": "0.17", "aqua:pypa/pipx": "1.18",
		"ripgrep": "15.2", "jq": "1.8", "yq": "4.54", "shellcheck": "0.11",
	}}))
	configSet := mise.ConfigSet{Tools: rts, Packages: all.Packages()}
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
