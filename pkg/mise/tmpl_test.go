package mise

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/slug"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

func TestSeedConfig(t *testing.T) {
	testutil.Home(t)

	require.NoError(t, SeedConfig(UserConfigPath(), nil))

	// the seed is the given tools' carrier — nil renders an empty region
	body, err := os.ReadFile(UserConfigPath()) // #nosec G304 -- the test's own seeded path
	require.NoError(t, err)
	assert.Equal(t, string(must.Get(renderConfig(nil))), string(body))
}

func TestSeedConfig_SkipsExisting(t *testing.T) {
	testutil.Home(t)
	require.NoError(t, os.MkdirAll(userdir.Dir(), osutil.Dir))
	require.NoError(t, os.WriteFile(UserConfigPath(), []byte("custom"), osutil.File))

	require.NoError(t, SeedConfig(UserConfigPath(), nil))

	body, err := os.ReadFile(UserConfigPath()) // #nosec G304 -- the test's own config path
	require.NoError(t, err)
	assert.Equal(t, "custom", string(body)) // the user's own file is never touched
}

// testTools is the fixture render's stand-in for a full prov resolution: one plain
// binding and one with a postinstall — the two shapes row renders
var testTools = []Tool{
	{Tool: "go", Version: "1.26"},
	{Tool: "python", Version: "3.13", PostInstall: "python -m pip install setuptools==82.0.1 wheel==0.47.0"},
}

// TestRenderConfig pins the fixture render to its golden
func TestRenderConfig(t *testing.T) {
	assert.Equal(t, golden(t, "TestRenderConfig.toml"), seedRender(t, testTools))
}

// TestRenderConfig_IndependentTools pins the render when the picker's whole
// second checklist rides the seed
func TestRenderConfig_IndependentTools(t *testing.T) {
	assert.Equal(t, golden(t, "TestRenderConfig_IndependentTools.toml"), seedRender(t, independentTools))
}

func TestSyncTools(t *testing.T) {
	// narrowedTools trims the tools like a spec selection
	narrowedTools := []Tool{
		{Tool: "go", Version: "1.26"},
		{Tool: "golangci-lint", Version: "2.13.1"},
	}

	// the marked region's rows after the sync, through the region's closing newline —
	// every case syncs narrowedTools
	const syncedRegion = `go = "1.26"
golangci-lint = "2.13.1"
`

	t.Run("replaces only the marked region", func(t *testing.T) {
		seedUser(t)

		require.NoError(t, SyncTools(t.TempDir(), narrowedTools))

		body := userBody(t)
		assert.Contains(t, body, syncedRegion)
		// the body minus the synced rows is the fixture's top and bottom: the dropped
		// tools' rows went with the region; the hand-written rows below stayed
		assert.Equal(t, golden(t, "TestSyncTools.toml"), strings.Replace(body, syncedRegion, "", 1))
	})

	t.Run("hand edits outside the region survive", func(t *testing.T) {
		seedUser(t)
		edited := strings.Replace(userBody(t), `shellcheck = "0.11.0"`, `shellcheck = "0.12.0"`, 1)
		require.NoError(t, os.WriteFile(UserConfigPath(), []byte(edited), osutil.File)) // #nosec G703 -- the test's own config path

		require.NoError(t, SyncTools(t.TempDir(), narrowedTools))

		body := userBody(t)
		assert.Contains(t, body, `shellcheck = "0.12.0"`)
		// the fixture with the same hand edit
		want := strings.Replace(golden(t, "TestSyncTools.toml"), `shellcheck = "0.11.0"`, `shellcheck = "0.12.0"`, 1)
		assert.Equal(t, want, strings.Replace(body, syncedRegion, "", 1))
	})

	t.Run("picks the project config over the user's", func(t *testing.T) {
		seedUser(t)
		projectDir := t.TempDir()
		marked := "[tools]\n# ccbox:tools-begin\nnode = \"26\"\n# ccbox:tools-end\n"
		require.NoError(t, os.WriteFile(ProjectConfigPath(projectDir), []byte(marked), osutil.File))

		require.NoError(t, SyncTools(projectDir, narrowedTools))

		project, err := os.ReadFile(ProjectConfigPath(projectDir)) // #nosec G304 -- the test's own config path
		require.NoError(t, err)
		assert.Contains(t, string(project), syncedRegion, "the project config's region syncs")
		assert.Equal(t, "[tools]\n# ccbox:tools-begin\n# ccbox:tools-end\n", strings.Replace(string(project), syncedRegion, "", 1))
		assert.NotContains(t, userBody(t), "golangci-lint", "the user config is left untouched")
	})

	t.Run("errors when neither is present", func(t *testing.T) {
		testutil.Home(t)

		err := SyncTools(t.TempDir(), narrowedTools)
		require.ErrorContains(t, err, "no mise config to sync")
		assert.NoFileExists(t, UserConfigPath(), "nothing is seeded")
	})

	t.Run("a legacy no-marker config errors untouched", func(t *testing.T) {
		testutil.Home(t)
		require.NoError(t, os.MkdirAll(userdir.Dir(), osutil.Dir))
		legacy := "[tools]\nnode = \"26\"\n" // seeded before the markers existed
		require.NoError(t, os.WriteFile(UserConfigPath(), []byte(legacy), osutil.File))

		err := SyncTools(t.TempDir(), narrowedTools)
		require.ErrorContains(t, err, "delete the file to reseed it")

		assert.Equal(t, legacy, userBody(t), "untouched")
	})

	t.Run("the lock goes stale for the next GenerateLock", func(t *testing.T) {
		seedUser(t)

		require.NoError(t, SyncTools(t.TempDir(), narrowedTools))

		expander := osutil.Expander{
			Source:    UserConfigPath(),
			Expansion: LockPath(UserConfigPath()),
			SHAPath:   filepath.Join(userdir.Tmp(), "misesha", slug.Path(UserConfigPath())+".sha256"),
		}
		isStale, err := expander.IsStale()
		require.NoError(t, err)
		assert.True(t, isStale, "the lock must regenerate after the sync")
	})
}

// userBody reads the user config's body
func userBody(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(UserConfigPath()) // #nosec G304 -- the test's own config path
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

// seedRender seeds the user-level config with tools, returning the seeded body
func seedRender(t *testing.T, tools []Tool) string {
	t.Helper()
	testutil.Home(t)
	require.NoError(t, os.MkdirAll(userdir.Dir(), osutil.Dir))
	require.NoError(t, SeedConfig(UserConfigPath(), tools))
	body, err := os.ReadFile(UserConfigPath()) // #nosec G304 -- the test's own seeded path
	require.NoError(t, err)
	return string(body)
}

// renderConfig executes the tmpl for tools — SeedConfig's content
func renderConfig(tools []Tool) ([]byte, error) {
	var b bytes.Buffer
	if err := configTmpl.Execute(&b, tools); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
