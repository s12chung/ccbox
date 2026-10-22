package mise

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/kit/miseutil"
	"github.com/s12chung/ccbox/pkg/kit/pypi"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/slug"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

// stubVersions serves the fixture lists at the versions service's data paths
func stubVersions(t *testing.T, lists map[string]string) {
	t.Helper()
	mux := http.NewServeMux()
	for path, list := range lists {
		mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(list))
		})
	}
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	miseutil.VersionsURL = server.URL
	t.Cleanup(func() { miseutil.VersionsURL = miseutil.VersionsBaseURL })
}

// versionList renders a versions-service TOML list dating every release
func versionList(versions map[string]time.Time) string {
	var list strings.Builder
	list.WriteString("[versions]\n")
	for _, version := range slices.Sorted(maps.Keys(versions)) {
		fmt.Fprintf(&list, "%q = { created_at = %s }\n", version, versions[version].UTC().Format(time.RFC3339Nano))
	}
	return list.String()
}

// stubPyPI serves the fixture indexes at PyPI's JSON paths
func stubPyPI(t *testing.T, indexes map[string]string) {
	t.Helper()
	mux := http.NewServeMux()
	for path, index := range indexes {
		mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(index))
		})
	}
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	pypi.JSONURL = server.URL
	t.Cleanup(func() { pypi.JSONURL = pypi.BaseURL })
}

// pins maps the resolved tools to their pins
func pins(rts []ResolvedTool) map[string]string {
	m := make(map[string]string, len(rts))
	for _, rt := range rts {
		m[rt.Name] = rt.Pin
	}
	return m
}

// pinned serves the map as the version func
func pinned(latests map[string]string) func(string) (string, error) {
	return func(name string) (string, error) { return latests[name], nil }
}

// resolvedConfig resolves the tools, wrapping them with the packages rows
func resolvedConfig(tools []Tool, state ToolState, packages []string) (ConfigSet, error) {
	rts, err := ResolvedTools(tools, state)
	if err != nil {
		return ConfigSet{}, err
	}
	return ConfigSet{Tools: rts, Packages: packages}, nil
}

func TestResolvedTools(t *testing.T) {
	t.Run("a tool new to the config fetches its latest, pinned to the minor", func(t *testing.T) {
		stubs := map[string]string{
			"/data/go.toml": versionList(map[string]time.Time{
				"1.26.5": time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), "1.27.2": time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
			}),
			"/data/node.toml": versionList(map[string]time.Time{
				"26.11.0": time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), "26.12.0": time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
			}),
		}
		stubVersions(t, stubs)

		rts, err := ResolvedTools([]Tool{{Name: "node"}, {Name: "go"}}, ToolState{
			VersionFunc: func(name string) (string, error) { return miseutil.LatestVersion(t.Context(), name) },
		})

		require.NoError(t, err)
		assert.Equal(t, map[string]string{"go": "1.27", "node": "26.12"}, pins(rts))
	})

	t.Run("a tool pinned in the config keeps its pin as written", func(t *testing.T) {
		rts, err := ResolvedTools([]Tool{{Name: "go"}, {Name: "node"}}, ToolState{
			Pins: map[string]string{"go": "1.24.1"}, VersionFunc: pinned(map[string]string{"node": "26.12"}),
		})

		require.NoError(t, err)
		assert.Equal(t, map[string]string{"go": "1.24.1", "node": "26.12"}, pins(rts))
	})

	t.Run("an all-pinned config never fetches", func(t *testing.T) {
		rts, err := ResolvedTools([]Tool{{Name: "go"}}, ToolState{Pins: map[string]string{"go": "1.25"}})

		require.NoError(t, err)
		assert.Equal(t, map[string]string{"go": "1.25"}, pins(rts))
	})

	t.Run("the postinstall's packages fill their full release", func(t *testing.T) {
		rts, err := ResolvedTools([]Tool{
			{Name: "node"},
			{Name: "python", PostInstallCmdTmpl: pipCmdTmpl, PostInstallTools: pipTools},
		}, ToolState{
			Pins: map[string]string{"python": "3.13"}, // python keeps its pin
			VersionFunc: pinned(map[string]string{
				"node": "26.12", "pypi:setuptools": "84.0.0", "pypi:wheel": "0.48.0",
			}),
		})

		require.NoError(t, err)
		assert.Equal(t, map[string]string{"node": "26.12", "python": "3.13"}, pins(rts))
		assert.Equal(t, "python -m pip install --no-cache-dir setuptools==84.0.0 wheel==0.48.0", rts[1].PostInstall,
			"the full releases — pip's == matches an exact version")
	})

	t.Run("a written postinstall is used as-is — its packages never fetch", func(t *testing.T) {
		rts, err := ResolvedTools([]Tool{{Name: "python", PostInstallCmdTmpl: pipCmdTmpl, PostInstallTools: pipTools}},
			ToolState{
				Pins:         map[string]string{"python": "3.13"},
				PostInstalls: map[string]string{"python": "python -m pip install --no-cache-dir setuptools==70.0.0"},
			})

		require.NoError(t, err)
		assert.Equal(t, map[string]string{"python": "3.13"}, pins(rts))
		assert.Equal(t, "python -m pip install --no-cache-dir setuptools==70.0.0", rts[0].PostInstall,
			"the written postinstall carries its pins — none resolve")
	})

	t.Run("a fetch error propagates", func(t *testing.T) {
		_, err := ResolvedTools([]Tool{{Name: "go"}}, ToolState{VersionFunc: func(string) (string, error) {
			return "", errors.New("the versions service is down")
		}})

		require.ErrorContains(t, err, "the versions service is down")
	})
}

func TestSeedConfig(t *testing.T) {
	testutil.Home(t)

	require.NoError(t, SeedConfig(UserConfigPath(), ConfigSet{}))

	// the seed is the given config set's carrier — the zero value renders empty regions
	body, err := os.ReadFile(UserConfigPath()) // #nosec G304 -- the test's own seeded path
	require.NoError(t, err)
	assert.Equal(t, string(must.Get(renderConfig(ConfigSet{}))), string(body))
}

func TestSeedConfig_SkipsExisting(t *testing.T) {
	testutil.Home(t)
	require.NoError(t, os.MkdirAll(userdir.Dir(), osutil.Dir))
	require.NoError(t, os.WriteFile(UserConfigPath(), []byte("custom"), osutil.File))

	require.NoError(t, SeedConfig(UserConfigPath(), ConfigSet{}))

	body, err := os.ReadFile(UserConfigPath()) // #nosec G304 -- the test's own config path
	require.NoError(t, err)
	assert.Equal(t, "custom", string(body)) // the user's own file is never touched
}

// testTools is the fixture render's tool list, one per row shape
var testTools = []Tool{
	{Name: "go"},
	{Name: "python", PostInstallCmdTmpl: pipCmdTmpl, PostInstallTools: pipTools},
}

// testConfigSet is testTools fully resolved, plus the packages rows
var testConfigSet = must.Get(resolvedConfig(testTools, ToolState{Pins: map[string]string{
	"go": "1.26", "python": "3.13",
	"pypi:setuptools": "82.0.1", "pypi:wheel": "0.47.0",
}}, []string{"libatomic1", "libyaml-0-2"}))

// TestRenderConfig pins the fixture render to its golden
func TestRenderConfig(t *testing.T) {
	assert.Equal(t, golden(t, "TestRenderConfig.toml"), seedRender(t, testConfigSet))
}

// TestRenderConfig_WrittenPostInstall pins the render: the written postinstall renders as-is
func TestRenderConfig_WrittenPostInstall(t *testing.T) {
	configSet := must.Get(resolvedConfig(testTools, ToolState{
		Pins: map[string]string{
			"go": "1.26", "python": "3.13",
			"pypi:setuptools": "82.0.1", "pypi:wheel": "0.47.0", // present yet bypassed — the written post wins
		},
		PostInstalls: map[string]string{"python": "python -m pip install --no-cache-dir setuptools==70.0.0"},
	}, testConfigSet.Packages))

	body := seedRender(t, configSet)

	want := strings.Replace(golden(t, "TestRenderConfig.toml"),
		`postinstall = "python -m pip install --no-cache-dir setuptools==82.0.1 wheel==0.47.0"`,
		`postinstall = "python -m pip install --no-cache-dir setuptools==70.0.0"`, 1)
	assert.Equal(t, want, body)
}

// TestRenderConfig_IndependentTools pins the render when only the independent
// tools are seeded, the dev-workflow CLIs at the tail
func TestRenderConfig_IndependentTools(t *testing.T) {
	configSet := must.Get(resolvedConfig(slices.Concat(independentTools, devTools), ToolState{Pins: map[string]string{
		"bats": "1.14", "hadolint": "2.15", "npm:playwright": "1.64",
		"ripgrep": "15.2", "jq": "1.8", "yq": "4.54", "shellcheck": "0.11",
	}}, nil))
	assert.Equal(t, golden(t, "TestRenderConfig_IndependentTools.toml"), seedRender(t, configSet))
}

func TestSyncTools(t *testing.T) {
	// narrowedSet trims the tools like a spec's runtimes, with its own packages pick
	narrowedSet := must.Get(resolvedConfig([]Tool{{Name: "go"}, {Name: "golangci-lint"}},
		ToolState{Pins: map[string]string{"go": "1.26", "golangci-lint": "2.13.1"}},
		[]string{"libatomic1", "libyaml-0-2"}))

	// the marked regions' rows after the sync, through each region's closing
	// newline — every case syncs narrowedSet
	const (
		syncedToolsRegion = `go = "1.26"
golangci-lint = "2.13.1"
`
		syncedPackagesRegion = `"apt:libatomic1" = "latest"
"apt:libyaml-0-2" = "latest"
`
	)

	t.Run("replaces only the marked regions", func(t *testing.T) {
		seedUser(t)

		require.NoError(t, Sync(t.TempDir(), narrowedSet))

		body := userBody(t)
		assert.Contains(t, body, syncedToolsRegion)
		assert.Contains(t, body, syncedPackagesRegion)
		// the body minus the synced rows is the fixture's top and bottom: the dropped
		// tools' rows went with the region; the hand-written rows below stayed
		assert.Equal(t, golden(t, "TestSyncTools.toml"), strings.Replace(body, syncedToolsRegion, "", 1))
	})

	t.Run("hand edits outside the region survive", func(t *testing.T) {
		seedUser(t)
		// a hand-written row between the regions — outside both
		edited := strings.Replace(userBody(t), "# ccbox:tools-end", "# ccbox:tools-end\nfzf = \"0.44.1\"", 1)
		require.NoError(t, os.WriteFile(UserConfigPath(), []byte(edited), osutil.File)) // #nosec G703 -- the test's own config path

		require.NoError(t, Sync(t.TempDir(), narrowedSet))

		body := userBody(t)
		assert.Contains(t, body, `fzf = "0.44.1"`)
		// the fixture with the same hand edit
		want := strings.Replace(golden(t, "TestSyncTools.toml"), "# ccbox:tools-end", "# ccbox:tools-end\nfzf = \"0.44.1\"", 1)
		assert.Equal(t, want, strings.Replace(body, syncedToolsRegion, "", 1))
	})

	t.Run("picks the project config over the user's", func(t *testing.T) {
		seedUser(t)
		projectDir := t.TempDir()
		marked := "[tools]\n# ccbox:tools-begin\nnode = \"26\"\n# ccbox:tools-end\n\n[bootstrap.packages]\n# ccbox:packages-begin\n# ccbox:packages-end\n"
		require.NoError(t, os.WriteFile(ProjectConfigPath(projectDir), []byte(marked), osutil.File))

		require.NoError(t, Sync(projectDir, narrowedSet))

		project, err := os.ReadFile(ProjectConfigPath(projectDir)) // #nosec G304 -- the test's own config path
		require.NoError(t, err)
		assert.Contains(t, string(project), syncedToolsRegion, "the project config's regions sync")
		assert.Contains(t, string(project), syncedPackagesRegion, "the project config's regions sync")
		assert.Equal(t, "[tools]\n# ccbox:tools-begin\n# ccbox:tools-end\n\n[bootstrap.packages]\n# ccbox:packages-begin\n# ccbox:packages-end\n",
			strings.Replace(strings.Replace(string(project), syncedToolsRegion, "", 1), syncedPackagesRegion, "", 1))
		assert.NotContains(t, userBody(t), "golangci-lint", "the user config is left untouched")
	})

	t.Run("errors when neither is present", func(t *testing.T) {
		testutil.Home(t)

		err := Sync(t.TempDir(), narrowedSet)
		require.ErrorContains(t, err, "no mise config to sync")
		assert.NoFileExists(t, UserConfigPath(), "nothing is seeded")
	})

	t.Run("a legacy no-marker config errors untouched", func(t *testing.T) {
		testutil.Home(t)
		require.NoError(t, os.MkdirAll(userdir.Dir(), osutil.Dir))
		legacy := "[tools]\nnode = \"26\"\n" // seeded before the markers existed
		require.NoError(t, os.WriteFile(UserConfigPath(), []byte(legacy), osutil.File))

		err := Sync(t.TempDir(), narrowedSet)
		require.ErrorContains(t, err, "delete the file to reseed it")

		assert.Equal(t, legacy, userBody(t), "untouched")
	})

	t.Run("a legacy config's outside rows never duplicate into the region", func(t *testing.T) {
		seedUser(t)
		// a config written before the dev CLIs rode the region: their rows sit outside it
		legacy := strings.Replace(userBody(t), "# ccbox:tools-end", "# ccbox:tools-end\ngo = \"1.26\"", 1)
		require.NoError(t, os.WriteFile(UserConfigPath(), []byte(legacy), osutil.File)) // #nosec G703 -- the test's own config path

		err := Sync(t.TempDir(), narrowedSet)

		require.ErrorContains(t, err, "delete the file to reseed it")
		assert.Equal(t, legacy, userBody(t), "untouched")
	})

	t.Run("the lock goes stale for the next GenerateLock", func(t *testing.T) {
		seedUser(t)

		require.NoError(t, Sync(t.TempDir(), narrowedSet))

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

// seedRender seeds the user-level config with the config set, returning the
// seeded body
func seedRender(t *testing.T, configSet ConfigSet) string {
	t.Helper()
	testutil.Home(t)
	require.NoError(t, os.MkdirAll(userdir.Dir(), osutil.Dir))
	require.NoError(t, SeedConfig(UserConfigPath(), configSet))
	body, err := os.ReadFile(UserConfigPath()) // #nosec G304 -- the test's own seeded path
	require.NoError(t, err)
	return string(body)
}

// renderConfig executes the tmpl for the config set — SeedConfig's content
func renderConfig(configSet ConfigSet) ([]byte, error) {
	var b bytes.Buffer
	if err := configTmpl.Execute(&b, configSet); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
