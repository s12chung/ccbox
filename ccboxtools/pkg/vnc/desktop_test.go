package vnc

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkger/artifact"
)

func TestStartDesktop_Missing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	err := StartDesktop("1600x900")

	require.Error(t, err)
	assert.ErrorIs(t, err, exec.ErrNotFound)
}

func TestStartDesktop_StartFails(t *testing.T) {
	// the script's interpreter is missing — LookPath finds it, the exec doesn't
	dir := t.TempDir()
	script := filepath.Join(dir, desktopBin)
	require.NoError(t, os.WriteFile(script, []byte("#!/nonexistent-interpreter\n"), artifact.ExecFileMode))
	t.Setenv("PATH", dir)

	err := StartDesktop("1600x900")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "vnc requested")
}

// writeResolutionScript bakes a desktop script recording its $VNC_RESOLUTION to a
// file, returning the file's path
func writeResolutionScript(t *testing.T) string {
	dir := t.TempDir()
	resolutionFile := filepath.Join(t.TempDir(), "resolution")
	script := filepath.Join(dir, desktopBin)
	require.NoError(t, os.WriteFile(script,
		[]byte("#!/bin/sh\nprintf '%s' \"$VNC_RESOLUTION\" > "+resolutionFile+"\n"), artifact.ExecFileMode))
	t.Setenv("PATH", dir)
	return resolutionFile
}

func TestStartDesktop_Starts(t *testing.T) {
	resolutionFile := writeResolutionScript(t)

	require.NoError(t, StartDesktop("1600x900"))
	require.Eventually(t, func() bool {
		body, err := os.ReadFile(resolutionFile) //nolint:gosec // test fixture path
		return err == nil && string(body) == "1600x900"
	}, time.Second, time.Millisecond, "the desktop script must receive the run's resolution")
}

func TestStartDesktop_OverridesInheritedResolution(t *testing.T) {
	// a child's getenv reads the first of duplicate env entries — an inherited
	// VNC_RESOLUTION must not shadow the run's
	resolutionFile := writeResolutionScript(t)
	t.Setenv(vncResolutionEnv, "42x42")

	require.NoError(t, StartDesktop("1600x900"))
	require.Eventually(t, func() bool {
		body, err := os.ReadFile(resolutionFile) //nolint:gosec // test fixture path
		return err == nil && string(body) == "1600x900"
	}, time.Second, time.Millisecond, "the run's resolution must win over the inherited one")
}
