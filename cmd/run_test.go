package cmd

import (
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/perm"
)

const testWorkspace = "/work/myproj"

// stubSeedProjectFn swaps the project seed step for a test double and returns a restore func.
func stubSeedProjectFn(fn func(fs.FS, string) ([]string, error)) func() {
	orig := seedProjectFn
	seedProjectFn = fn
	return func() { seedProjectFn = orig }
}

func TestSafeSeedProjectDirMissingSeeds(t *testing.T) {
	cacheDir := t.TempDir()
	wantDir := projectDir(cacheDir, testWorkspace)

	var gotDir string
	called := false
	defer stubSeedProjectFn(func(_ fs.FS, dest string) ([]string, error) {
		called, gotDir = true, dest
		return nil, nil
	})()

	dir, err := safeSeedProjectDir(cacheDir, testWorkspace)
	require.NoError(t, err)
	assert.True(t, called, "seedProjectFn not called for missing dir")
	assert.Equal(t, wantDir, gotDir, "seeded dir")
	assert.Equal(t, wantDir, dir)
}

func TestSafeSeedProjectDirExistingSkips(t *testing.T) {
	cacheDir := t.TempDir()
	wantDir := projectDir(cacheDir, testWorkspace)
	require.NoError(t, os.MkdirAll(wantDir, perm.Dir))

	called := false
	defer stubSeedProjectFn(func(fs.FS, string) ([]string, error) {
		called = true
		return nil, nil
	})()

	dir, err := safeSeedProjectDir(cacheDir, testWorkspace)
	require.NoError(t, err)
	assert.False(t, called, "seedProjectFn called for existing dir")
	assert.Equal(t, wantDir, dir)
}

func TestSafeSeedProjectDirPropagatesSeedError(t *testing.T) {
	wantErr := errors.New("boom")
	defer stubSeedProjectFn(func(fs.FS, string) ([]string, error) {
		return nil, wantErr
	})()

	_, err := safeSeedProjectDir(t.TempDir(), testWorkspace)
	assert.ErrorIs(t, err, wantErr)
}
