package cmd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

// stubSeedTreeFn swaps the seed step for a test double and returns a restore func.
func stubSeedTreeFn(fn func(fs.FS, string) ([]string, error)) func() {
	orig := seedTreeFn
	seedTreeFn = fn
	return func() { seedTreeFn = orig }
}

// Each CLI seeds the merged tree (shared + per-CLI) into its config dir.
func TestSafeSeedConfig_MissingSeeds(t *testing.T) {
	cases := []struct {
		name    string
		cli     string
		ownFile string
	}{
		{name: "claude", cli: "claude", ownFile: "settings.json"},
		{name: "codex", cli: "codex", ownFile: "config.toml"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			userDir := t.TempDir()
			cli, requireOK := harness.For(tc.cli)
			require.True(t, requireOK)
			wantDir := filepath.Join(userDir, cli.Name)

			var gotDest string
			var gotFS fs.FS
			defer stubSeedTreeFn(func(fsys fs.FS, dest string) ([]string, error) {
				gotDest, gotFS = dest, fsys
				return nil, nil
			})()

			require.NoError(t, safeSeedCLIConfig(userDir, tc.cli, false))
			assert.Equal(t, wantDir, gotDest, "config dir seeded")

			var paths []string
			require.NoError(t, fs.WalkDir(gotFS, ".", func(p string, d fs.DirEntry, err error) error {
				require.NoError(t, err)
				if !d.IsDir() {
					paths = append(paths, p)
				}
				return nil
			}))
			assert.Contains(t, paths, cli.SeedAgentsFilename, "shared AGENTS doc renamed into place")
			assert.Contains(t, paths, tc.ownFile, "per-CLI tree merged in")
		})
	}
}

func TestSafeSeedConfig_ExistingSkips(t *testing.T) {
	userDir := t.TempDir()
	configDir := filepath.Join(userDir, "claude")
	require.NoError(t, os.MkdirAll(configDir, ioutil.Dir))

	called := false
	defer stubSeedTreeFn(func(fs.FS, string) ([]string, error) {
		called = true
		return nil, nil
	})()

	require.NoError(t, safeSeedCLIConfig(userDir, "claude", false))
	assert.False(t, called, "seed fn called for existing dir without confirm")
}

func TestSafeSeedConfig_PropagatesSeedError(t *testing.T) {
	wantErr := errors.New("boom")
	defer stubSeedTreeFn(func(fs.FS, string) ([]string, error) {
		return nil, wantErr
	})()

	assert.ErrorIs(t, safeSeedCLIConfig(t.TempDir(), "claude", false), wantErr)
}
