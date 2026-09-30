package entrypoint

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkger/artifact"
)

func TestStartDesktop_Missing(t *testing.T) {
	missing := func(string) (string, error) { return "", exec.ErrNotFound }

	assert.False(t, startDesktop(missing))
}

func TestStartDesktop_StartFails(t *testing.T) {
	// lookPath finds the binary, but it's gone by start time — warns, doesn't fail
	present := func(string) (string, error) { return "/usr/local/bin/desktop", nil }
	t.Setenv("PATH", t.TempDir())

	assert.False(t, startDesktop(present))
}

func TestStartDesktop_Starts(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, desktopBin)
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), artifact.ExecFileMode))
	t.Setenv("PATH", dir)

	assert.True(t, startDesktop(exec.LookPath))
}
