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
	"github.com/s12chung/ccbox/pkg/perm"
)

// stubSeedTreeFn swaps the seed step for a test double and returns a restore func.
func stubSeedTreeFn(fn func(fs.FS, string, map[string]string) ([]string, error)) func() {
	orig := seedTreeFn
	seedTreeFn = fn
	return func() { seedTreeFn = orig }
}

// Each CLI seeds its own config dir leaf with its own renames.
func TestSafeSeedConfigMissingSeeds(t *testing.T) {
	cases := []struct {
		name string
		cli  harness.Name
	}{
		{"claude", harness.NameClaude},
		{"codex", harness.NameCodex},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cacheDir := t.TempDir()
			spec, requireOK := harness.For(tc.cli)
			require.True(t, requireOK)
			wantDir := filepath.Join(cacheDir, spec.SeedSrcFolder)

			var gotDir string
			var gotRenames map[string]string
			called := false
			defer stubSeedTreeFn(func(_ fs.FS, dest string, renames map[string]string) ([]string, error) {
				called, gotDir, gotRenames = true, dest, renames
				return nil, nil
			})()

			dir, err := safeSeedConfig(cacheDir, tc.cli, false)
			require.NoError(t, err)
			assert.True(t, called, "seed fn not called for missing dir")
			assert.Equal(t, wantDir, gotDir, "seeded dir")
			assert.Equal(t, spec.SeedRenames, gotRenames, "seed renames")
			assert.Equal(t, wantDir, dir)
		})
	}
}

func TestSafeSeedConfigExistingSkips(t *testing.T) {
	cacheDir := t.TempDir()
	configDir := filepath.Join(cacheDir, "claude-config")
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
