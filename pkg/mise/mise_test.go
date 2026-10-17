package mise

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/slug"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

// The committed config fixture: the project config's body and the lock body
// commitLock writes for it.
const (
	fixtureConfig = "[tools]\nnode = \"26\"\n"
	fixtureLock   = "lock"
)

func TestImageRefFromDockerfile(t *testing.T) {
	dockerfile := "FROM debian:trixie-slim AS base\n\nFROM ghcr.io/jdx/mise:2026.9.6 AS mise\n\nFROM base\n"
	imageRef, err := ImageRefFromDockerfile([]byte(dockerfile))
	require.NoError(t, err)
	assert.Equal(t, "ghcr.io/jdx/mise:2026.9.6", imageRef)
}

func TestImageRefFromDockerfile_MissingFrom(t *testing.T) {
	dockerfile := "FROM debian:trixie-slim AS base\nFROM ghcr.io/jdx/mise:2026.9.6\n"
	_, err := ImageRefFromDockerfile([]byte(dockerfile))
	assert.ErrorContains(t, err, "no `FROM <image> AS mise`")
}

func TestGenerateLock_NoConfig(t *testing.T) {
	testutil.Home(t)
	projectDir := t.TempDir()
	require.NoError(t, GenerateLock(t.Context(), "image", ProjectConfigPath(projectDir)))
	_, err := os.Stat(LockPath(ProjectConfigPath(projectDir)))
	require.ErrorIs(t, err, os.ErrNotExist, "no config, no lock")
}

func TestGenerateLock_FreshLock(t *testing.T) {
	testutil.Home(t)
	configPath := ProjectConfigPath(t.TempDir())
	require.NoError(t, os.WriteFile(configPath, []byte(fixtureConfig), osutil.File))
	require.NoError(t, commitLock(t, configPath))

	// a fresh lock skips the docker run, so this passes without a daemon
	require.NoError(t, GenerateLock(t.Context(), "image", configPath))
	lock, err := os.ReadFile(LockPath(configPath))
	require.NoError(t, err)
	assert.Equal(t, fixtureLock, string(lock), "a fresh lock is left untouched")
}

func TestGenerateLock_SHAUnderTmp(t *testing.T) {
	testutil.Home(t)
	configPath := ProjectConfigPath(t.TempDir())
	require.NoError(t, os.WriteFile(configPath, []byte(fixtureConfig), osutil.File))
	require.NoError(t, commitLock(t, configPath))

	shaPath := filepath.Join(userdir.Tmp(), "misesha", slug.Path(configPath)+".sha256")
	require.FileExists(t, shaPath, "the sha rides under userdir's tmp")
	_, err := os.Stat(configPath + ".sha256")
	require.ErrorIs(t, err, os.ErrNotExist, "the config's dir carries no sha")
}

func TestGenerateLock_UserConfig(t *testing.T) {
	seedUser(t)

	// a fresh lock skips the docker run, so this passes without a daemon
	require.NoError(t, GenerateLock(t.Context(), "image", UserConfigPath()))
	lock, err := os.ReadFile(LockPath(UserConfigPath()))
	require.NoError(t, err)
	assert.Equal(t, fixtureLock, string(lock), "a fresh lock is left untouched")
}

// commitLock writes both lock artifacts for the config on disk — the state GenerateLock
// leaves behind, without the docker run its test can't make.
func commitLock(t *testing.T, configPath string) error {
	t.Helper()
	expander := osutil.Expander{
		Source:    configPath,
		Expansion: LockPath(configPath),
		SHAPath:   filepath.Join(userdir.Tmp(), "misesha", slug.Path(configPath)+".sha256"),
	}
	return expander.Expand([]byte(fixtureLock))
}

func TestReplaceSidecars(t *testing.T) {
	testutil.Home(t)
	configPath := ProjectConfigPath(t.TempDir())

	t.Run("a generation replaces the previous one", func(t *testing.T) {
		require.NoError(t, replaceSidecars(configPath, map[string][]byte{
			"npm-tool/1.0.0/aube-lock.yaml": []byte("old"),
		}))
		require.NoError(t, replaceSidecars(configPath, map[string][]byte{
			"npm-tool/1.1.0/aube-lock.yaml": []byte("new"),
		}))

		body, err := os.ReadFile(filepath.Join(SidecarsPath(configPath), "npm-tool/1.1.0/aube-lock.yaml"))
		require.NoError(t, err)
		assert.Equal(t, "new", string(body))
		_, err = os.Stat(filepath.Join(SidecarsPath(configPath), "npm-tool/1.0.0"))
		require.ErrorIs(t, err, os.ErrNotExist, "the previous generation's dir is gone")
	})

	t.Run("an empty generation clears the sidecars", func(t *testing.T) {
		require.NoError(t, replaceSidecars(configPath, nil))
		_, err := os.Stat(SidecarsPath(configPath))
		require.ErrorIs(t, err, os.ErrNotExist)
	})
}

// commitSidecars writes the config's sidecar files — the state replaceSidecars
// leaves behind
func commitSidecars(t *testing.T, configPath string, sidecars map[string][]byte) {
	t.Helper()
	for _, rel := range slices.Sorted(maps.Keys(sidecars)) {
		require.NoError(t, osutil.SafeWriteFile(filepath.Join(SidecarsPath(configPath), rel), sidecars[rel]))
	}
}

// seedUser seeds the user-level config and commits its lock
func seedUser(t *testing.T) {
	t.Helper()
	testutil.Home(t)
	require.NoError(t, SeedConfig(UserConfigPath(), ConfigSet{}))
	require.NoError(t, commitLock(t, UserConfigPath()))
}

// embedFSFixture mirrors the raw embed: no docker/mise — BuildFS injects it.
func embedFSFixture() fs.FS {
	return fstest.MapFS{
		"Dockerfile":                {Data: []byte("FROM base\nCOPY docker/mise/ /etc/mise/\n")},
		"docker/desktop/desktop.sh": {Data: []byte("echo")},
	}
}

func TestIndependentTools(t *testing.T) {
	assert.Equal(t, independentTools, IndependentTools())

	// a clone: mutating the answer doesn't leak into the next call
	tools := IndependentTools()
	tools[0] = Tool{Tool: "bogus"}
	assert.Equal(t, independentTools, IndependentTools())
}

func TestBuildFS(t *testing.T) {
	testutil.Home(t) // the user level starts empty, like a fresh host

	projectDir := t.TempDir()
	require.NoError(t, os.WriteFile(ProjectConfigPath(projectDir), []byte(fixtureConfig), osutil.File))
	require.NoError(t, commitLock(t, ProjectConfigPath(projectDir)))

	tests := []struct {
		name       string
		seedUser   bool
		projectDir string
		wantErr    bool
		wantConfig string
		wantLock   string
	}{
		// rootSeed seeds the user level on every ccbox invocation, so no config is
		// an error: there is nothing to build from
		{name: "no config", projectDir: t.TempDir(), wantErr: true},
		{name: "user", seedUser: true, projectDir: t.TempDir(), wantConfig: string(must.Get(renderConfig(ConfigSet{}))), wantLock: fixtureLock},
		// the user level is seeded here too: the project's own config wins
		{name: "project", seedUser: true, projectDir: projectDir, wantConfig: fixtureConfig, wantLock: fixtureLock},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.seedUser {
				seedUser(t)
			}
			merged, err := BuildFS(embedFSFixture(), tt.projectDir)
			if tt.wantErr {
				require.ErrorContains(t, err, "no mise config to build from")
				return
			}
			require.NoError(t, err)

			config, err := fs.ReadFile(merged, contextConfigPath)
			require.NoError(t, err)
			assert.Equal(t, tt.wantConfig, string(config), "the config at docker/mise/ is the flow's own")

			lock, err := fs.ReadFile(merged, contextLockPath)
			if tt.wantLock == "" {
				require.Error(t, err, "the default flow embeds no lock")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantLock, string(lock))
			}

			_, err = fs.ReadFile(merged, "Dockerfile")
			require.NoError(t, err, "the rest of the embed rides untouched")
		})
	}
}

func TestBuildFS_Sidecars(t *testing.T) {
	t.Run("the committed sidecars ride the context", func(t *testing.T) {
		testutil.Home(t)
		configPath := ProjectConfigPath(t.TempDir())
		require.NoError(t, os.WriteFile(configPath, []byte(fixtureConfig), osutil.File))
		require.NoError(t, commitLock(t, configPath))
		commitSidecars(t, configPath, map[string][]byte{"npm-tool/1.0.0/aube-lock.yaml": []byte("graph")})

		merged, err := BuildFS(embedFSFixture(), filepath.Dir(configPath))
		require.NoError(t, err)
		body, err := fs.ReadFile(merged, contextSidecarsPath+"/npm-tool/1.0.0/aube-lock.yaml")
		require.NoError(t, err)
		assert.Equal(t, "graph", string(body))
	})

	t.Run("no sidecars, no context entries", func(t *testing.T) {
		seedUser(t)

		merged, err := BuildFS(embedFSFixture(), t.TempDir())
		require.NoError(t, err)
		_, err = fs.Stat(merged, contextSidecarsPath)
		require.ErrorIs(t, err, fs.ErrNotExist, "the context carries no sidecar tree")
	})
}

// toolsConfig is the fixture config: plain bindings, an inline table and a
// sub-table — the shapes a [tools] table carries
const toolsConfig = `[tools]
go = "1.26"
ruby = { version = "3.4" }

[tools."npm:playwright"]
version = "latest"
`

func TestToolNames(t *testing.T) {
	testutil.Home(t)
	projectDir := t.TempDir()
	require.NoError(t, os.WriteFile(ProjectConfigPath(projectDir), []byte(toolsConfig), osutil.File))

	names, err := ToolNames(projectDir)
	require.NoError(t, err)
	assert.Equal(t, []string{"go", "npm:playwright", "ruby"}, names, "the [tools] table's names, sub-tables included")
}

func TestToolNames_UserFallback(t *testing.T) {
	testutil.Home(t)
	require.NoError(t, SeedConfig(UserConfigPath(), ConfigSet{Tools: []Tool{{Tool: "node", Version: "26"}}}))

	names, err := ToolNames(t.TempDir())
	require.NoError(t, err)
	assert.Contains(t, names, "node", "the user config's [tools] names")
}

func TestToolNames_Errors(t *testing.T) {
	t.Run("no config", func(t *testing.T) {
		testutil.Home(t)

		_, err := ToolNames(t.TempDir())
		require.Error(t, err)
		assert.ErrorContains(t, err, "no mise config to read")
	})

	t.Run("broken toml", func(t *testing.T) {
		testutil.Home(t)
		configPath := ProjectConfigPath(t.TempDir())
		require.NoError(t, os.WriteFile(configPath, []byte("[tools\n"), osutil.File))

		_, err := ToolNames(t.TempDir())
		require.Error(t, err)
		assert.ErrorContains(t, err, "mise:")
	})
}
