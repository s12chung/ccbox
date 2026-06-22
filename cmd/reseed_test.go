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
)

// stubSeedClaudeFn swaps the seed step for a test double and returns a restore func.
func stubSeedClaudeFn(fn func(fs.FS, string) ([]string, error)) func() {
	orig := seedClaudeFn
	seedClaudeFn = fn
	return func() { seedClaudeFn = orig }
}

func TestSafeSeedClaudeConfigMissingSeeds(t *testing.T) {
	cacheDir := t.TempDir()
	wantDir := filepath.Join(cacheDir, "claude-config")

	var gotDir string
	called := false
	defer stubSeedClaudeFn(func(_ fs.FS, dest string) ([]string, error) {
		called, gotDir = true, dest
		return nil, nil
	})()

	dir, err := safeSeedClaudeConfig(cacheDir, false)
	require.NoError(t, err)
	assert.True(t, called, "seedClaudeFn not called for missing dir")
	assert.Equal(t, wantDir, gotDir, "seeded dir")
	assert.Equal(t, wantDir, dir)
}

func TestSafeSeedClaudeConfigExistingSkips(t *testing.T) {
	cacheDir := t.TempDir()
	configDir := filepath.Join(cacheDir, "claude-config")
	require.NoError(t, os.MkdirAll(configDir, perm.Dir))

	called := false
	defer stubSeedClaudeFn(func(fs.FS, string) ([]string, error) {
		called = true
		return nil, nil
	})()

	dir, err := safeSeedClaudeConfig(cacheDir, false)
	require.NoError(t, err)
	assert.False(t, called, "seedClaudeFn called for existing dir without confirm")
	assert.Equal(t, configDir, dir)
}

func TestSafeSeedClaudeConfigPropagatesSeedError(t *testing.T) {
	wantErr := errors.New("boom")
	defer stubSeedClaudeFn(func(fs.FS, string) ([]string, error) {
		return nil, wantErr
	})()

	_, err := safeSeedClaudeConfig(t.TempDir(), false)
	assert.ErrorIs(t, err, wantErr)
}
