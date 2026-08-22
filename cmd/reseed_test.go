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
	"github.com/s12chung/ccbox/pkg/util/perm"
)

// stubSeedTreeFn swaps the seed step for a test double and returns a restore func.
func stubSeedTreeFn(fn func(fs.FS, string, map[string]string) ([]string, error)) func() {
	orig := seedTreeFn
	seedTreeFn = fn
	return func() { seedTreeFn = orig }
}

// Each CLI seeds the shared tree first (its memory doc renamed into place), then its own.
func TestSafeSeedConfigMissingSeeds(t *testing.T) {
	cases := []struct {
		name      string
		cli       harness.Name
		memoryDst string
	}{
		{name: "claude", cli: harness.NameClaude, memoryDst: "CLAUDE.md"},
		{name: "codex", cli: harness.NameCodex, memoryDst: "AGENTS.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cacheDir := t.TempDir()
			cli, requireOK := harness.For(tc.cli)
			require.True(t, requireOK)
			wantDir := filepath.Join(cacheDir, string(cli.Name))

			type call struct {
				dest    string
				renames map[string]string
			}
			var calls []call
			defer stubSeedTreeFn(func(_ fs.FS, dest string, renames map[string]string) ([]string, error) {
				calls = append(calls, call{dest, renames})
				return nil, nil
			})()

			dir, err := safeSeedConfig(cacheDir, tc.cli, false)
			require.NoError(t, err)
			assert.Equal(t, wantDir, dir)
			assert.Equal(t, []call{
				{wantDir, map[string]string{harness.AgentsFileName: cli.SeedAgentsFilename}},
				{wantDir, nil},
			}, calls, "shared then per-CLI tree seeded into the config dir")
		})
	}
}

func TestSafeSeedConfigExistingSkips(t *testing.T) {
	cacheDir := t.TempDir()
	configDir := filepath.Join(cacheDir, "claude")
	require.NoError(t, os.MkdirAll(configDir, perm.Dir))

	called := false
	defer stubSeedTreeFn(func(fs.FS, string, map[string]string) ([]string, error) {
		called = true
		return nil, nil
	})()

	dir, err := safeSeedConfig(cacheDir, harness.NameClaude, false)
	require.NoError(t, err)
	assert.False(t, called, "seed fn called for existing dir without confirm")
	assert.Equal(t, configDir, dir)
}

func TestSafeSeedConfigPropagatesSeedError(t *testing.T) {
	wantErr := errors.New("boom")
	defer stubSeedTreeFn(func(fs.FS, string, map[string]string) ([]string, error) {
		return nil, wantErr
	})()

	_, err := safeSeedConfig(t.TempDir(), harness.NameClaude, false)
	assert.ErrorIs(t, err, wantErr)
}
