package mise

import (
	"io/fs"
	"os"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

// The committed config fixture: the project config's body and the lock body
// commitLock writes for it.
const (
	fixtureConfig = "[tools]\nnode = \"26\"\n"
	fixtureLock   = "lock"
)

func TestImageFromDockerfile(t *testing.T) {
	dockerfile := "FROM debian:trixie-slim AS base\n\nFROM ghcr.io/jdx/mise:2026.6.9 AS mise\n\nFROM base\n"
	image, err := ImageFromDockerfile([]byte(dockerfile))
	require.NoError(t, err)
	assert.Equal(t, "ghcr.io/jdx/mise:2026.6.9", image)
}

func TestImageFromDockerfile_MissingFrom(t *testing.T) {
	dockerfile := "FROM debian:trixie-slim AS base\nFROM ghcr.io/jdx/mise:2026.6.9\n"
	_, err := ImageFromDockerfile([]byte(dockerfile))
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

func TestGenerateLock_UserConfig(t *testing.T) {
	seedUser(t)

	// a fresh lock skips the docker run, so this passes without a daemon
	require.NoError(t, GenerateLock(t.Context(), "image", UserConfigPath()))
	lock, err := os.ReadFile(LockPath(UserConfigPath()))
	require.NoError(t, err)
	assert.Equal(t, fixtureLock, string(lock), "a fresh lock is left untouched")
}

func TestSeedConfig(t *testing.T) {
	testutil.Home(t)

	require.NoError(t, SeedConfig(UserConfigPath()))

	// the seed is the pinned defaults' carrier
	body, err := os.ReadFile(UserConfigPath()) // #nosec G304 -- the test's own seeded path
	require.NoError(t, err)
	assert.Equal(t, string(defaultConfig), string(body))
}

func TestSeedConfig_SkipsExisting(t *testing.T) {
	testutil.Home(t)
	require.NoError(t, os.MkdirAll(userdir.Dir(), osutil.Dir))
	require.NoError(t, os.WriteFile(UserConfigPath(), []byte("custom"), osutil.File))

	require.NoError(t, SeedConfig(UserConfigPath()))

	body, err := os.ReadFile(UserConfigPath()) // #nosec G304 -- the test's own config path
	require.NoError(t, err)
	assert.Equal(t, "custom", string(body)) // the user's own file is never touched
}

// commitLock writes both lock artifacts for the config on disk — the state GenerateLock
// leaves behind, without the docker run its test can't make.
func commitLock(t *testing.T, configPath string) error {
	t.Helper()
	return osutil.Expander{
		Source:    configPath,
		Expansion: LockPath(configPath),
	}.Expand([]byte(fixtureLock))
}

// seedUser seeds the user-level config and commits its lock
func seedUser(t *testing.T) {
	t.Helper()
	testutil.Home(t)
	require.NoError(t, SeedConfig(UserConfigPath()))
	require.NoError(t, commitLock(t, UserConfigPath()))
}

// embedFSFixture mirrors the raw embed: no docker/mise — BuildFS injects it.
func embedFSFixture() fs.FS {
	return fstest.MapFS{
		"Dockerfile":                {Data: []byte("FROM base\nCOPY docker/mise/ /etc/mise/\n")},
		"docker/desktop/desktop.sh": {Data: []byte("echo")},
	}
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
		wantErr    string
		wantConfig string
		wantLock   string
	}{
		// rootSeed seeds the user level before any build: no config guards a seed
		// gone missing, it's not a real flow
		{name: "no config", projectDir: t.TempDir(), wantErr: "no mise config found"},
		{name: "user", seedUser: true, projectDir: t.TempDir(), wantConfig: string(defaultConfig), wantLock: fixtureLock},
		// the user level is seeded here too: the project's own config wins
		{name: "project", seedUser: true, projectDir: projectDir, wantConfig: fixtureConfig, wantLock: fixtureLock},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.seedUser {
				seedUser(t)
			}
			merged, err := BuildFS(embedFSFixture(), tt.projectDir)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
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
