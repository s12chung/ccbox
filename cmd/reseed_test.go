package cmd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/perm"
	"github.com/s12chung/ccbox/pkg/projectcfg"
)

// stubSeedClaudeFn swaps the seed step for a test double and returns a restore func.
func stubSeedClaudeFn(fn func(fs.FS, string) ([]string, error)) func() {
	orig := seedClaudeFn
	seedClaudeFn = fn
	return func() { seedClaudeFn = orig }
}

// stubSeedCodexFn swaps the codex seed step for a test double and returns a restore func.
func stubSeedCodexFn(fn func(fs.FS, string) ([]string, error)) func() {
	orig := seedCodexFn
	seedCodexFn = fn
	return func() { seedCodexFn = orig }
}

// Each CLI seeds its own config dir via its own seed fn.
func TestSafeSeedConfigMissingSeeds(t *testing.T) {
	cases := []struct {
		name     string
		cli      projectcfg.CLI
		wantLeaf string
		stub     func(func(fs.FS, string) ([]string, error)) func()
	}{
		{"claude", projectcfg.CLIClaude, "claude-config", stubSeedClaudeFn},
		{"codex", projectcfg.CLICodex, "codex-config", stubSeedCodexFn},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cacheDir := t.TempDir()
			wantDir := filepath.Join(cacheDir, tc.wantLeaf)

			var gotDir string
			called := false
			defer tc.stub(func(_ fs.FS, dest string) ([]string, error) {
				called, gotDir = true, dest
				return nil, nil
			})()

			dir, err := safeSeedConfig(cacheDir, tc.cli, false)
			require.NoError(t, err)
			assert.True(t, called, "seed fn not called for missing dir")
			assert.Equal(t, wantDir, gotDir, "seeded dir")
			assert.Equal(t, wantDir, dir)
		})
	}
}

func TestSafeSeedConfigExistingSkips(t *testing.T) {
	cacheDir := t.TempDir()
	configDir := filepath.Join(cacheDir, "claude-config")
	require.NoError(t, os.MkdirAll(configDir, perm.Dir))

	called := false
	defer stubSeedClaudeFn(func(fs.FS, string) ([]string, error) {
		called = true
		return nil, nil
	})()

	dir, err := safeSeedConfig(cacheDir, projectcfg.CLIClaude, false)
	require.NoError(t, err)
	assert.False(t, called, "seedClaudeFn called for existing dir without confirm")
	assert.Equal(t, configDir, dir)
}

func TestSafeSeedConfigPropagatesSeedError(t *testing.T) {
	wantErr := errors.New("boom")
	defer stubSeedClaudeFn(func(fs.FS, string) ([]string, error) {
		return nil, wantErr
	})()

	_, err := safeSeedConfig(t.TempDir(), projectcfg.CLIClaude, false)
	assert.ErrorIs(t, err, wantErr)
}
