package vncdeps

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// execFileMode is the mode the fixture's chromium carries.
const execFileMode os.FileMode = 0o755

// getenv fakes the env a run sets, answering the browsers dir override
func getenv(dir string) func(string) string {
	return func(key string) string {
		if key == BrowsersPathEnv {
			return dir
		}
		return ""
	}
}

func TestCheckPlaywright_DefaultsPath(t *testing.T) {
	var pattern string
	glob := func(p string) ([]string, error) { pattern = p; return nil, nil }

	err := CheckPlaywright(getenv(""), glob)

	require.Error(t, err)
	assert.Equal(t, filepath.Join(DefaultBrowsersPath, chromeGlob), pattern)
	assert.Contains(t, err.Error(), "vnc requested")
	assert.Contains(t, err.Error(), "no playwright chromium under /opt/ms-playwright")
}

func TestCheckPlaywright_HonorsEnvOverride(t *testing.T) {
	require.NoError(t, CheckPlaywright(getenv(writeChrome(t)), filepath.Glob))
}

func TestCheckPlaywright_EmptyOverride(t *testing.T) {
	dir := t.TempDir()

	err := CheckPlaywright(getenv(dir), filepath.Glob)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no playwright chromium under "+dir)
}

func TestCheckPlaywright_GlobFails(t *testing.T) {
	glob := func(string) ([]string, error) { return nil, os.ErrPermission }

	err := CheckPlaywright(getenv(""), glob)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "find playwright chromium")
	assert.ErrorIs(t, err, os.ErrPermission)
}

// writeChrome bakes one playwright chromium under dir, as `playwright install` does
func writeChrome(t *testing.T) string {
	dir := t.TempDir()
	chrome := filepath.Join(dir, "chromium-1200", "chrome-linux", "chrome")
	require.NoError(t, os.MkdirAll(filepath.Dir(chrome), execFileMode))
	require.NoError(t, os.WriteFile(chrome, []byte("#!/bin/sh\n"), execFileMode))
	return dir
}
