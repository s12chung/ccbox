package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/util/fsutil"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

// stubSeedTreeFn swaps the seed step for a test double and returns a restore func.
func stubSeedTreeFn(fn func(fsutil.RenamedFS, string) ([]string, error)) func() {
	orig := seedTreeFn
	seedTreeFn = fn
	return func() { seedTreeFn = orig }
}

// Each CLI seeds the shared tree first (its memory doc renamed into place), then its own.
func TestSafeSeedConfigMissingSeeds(t *testing.T) {
	cases := []struct {
		name      string
		cli       string
		memoryDst string
	}{
		{name: "claude", cli: "claude", memoryDst: "CLAUDE.md"},
		{name: "codex", cli: "codex", memoryDst: "AGENTS.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			userDir := t.TempDir()
			cli, requireOK := harness.For(tc.cli)
			require.True(t, requireOK)
			wantDir := filepath.Join(userDir, cli.Name)

			type call struct {
				dest    string
				renames map[string]string
			}
			var calls []call
			defer stubSeedTreeFn(func(src fsutil.RenamedFS, dest string) ([]string, error) {
				calls = append(calls, call{dest, src.Renames})
				return nil, nil
			})()

			require.NoError(t, safeSeedCLIConfig(userDir, tc.cli, false))
			assert.Equal(t, []call{
				{wantDir, map[string]string{harness.AgentsFileName: cli.SeedAgentsFilename}},
				{wantDir, nil},
			}, calls, "shared then per-CLI tree seeded into the config dir")
		})
	}
}

func TestSafeSeedConfigExistingSkips(t *testing.T) {
	userDir := t.TempDir()
	configDir := filepath.Join(userDir, "claude")
	require.NoError(t, os.MkdirAll(configDir, ioutil.Dir))

	called := false
	defer stubSeedTreeFn(func(fsutil.RenamedFS, string) ([]string, error) {
		called = true
		return nil, nil
	})()

	require.NoError(t, safeSeedCLIConfig(userDir, "claude", false))
	assert.False(t, called, "seed fn called for existing dir without confirm")
}

func TestSafeSeedConfigPropagatesSeedError(t *testing.T) {
	wantErr := errors.New("boom")
	defer stubSeedTreeFn(func(fsutil.RenamedFS, string) ([]string, error) {
		return nil, wantErr
	})()

	assert.ErrorIs(t, safeSeedCLIConfig(t.TempDir(), "claude", false), wantErr)
}
