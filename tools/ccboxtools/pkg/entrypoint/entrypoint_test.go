package entrypoint

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkger/artifact"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/vnc/vncdeps"
)

func TestMain(m *testing.M) {
	os.Exit(testutil.MainHome(m, nil))
}

func TestExecArgv(t *testing.T) {
	tests := []struct {
		name    string
		argv    []string
		wantErr string
	}{
		{"empty", nil, "no command to exec"},
		{"missing binary", []string{"ccbox-definitely-not-a-binary"}, "exec ccbox-definitely-not-a-binary"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := execArgv(tt.argv)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// rpmInfo is Go-constructed only: the config's validation rejects the artifact type,
// so the install fails before any download
var rpmInfo = pkginfo.GUIPkgInfo{
	PkgInfo: pkginfo.PkgInfo{
		Name: "app",
		ReleaseURL: &pkginfo.ReleaseURL{
			URL:      "https://example.dev/manifest",
			JQSchema: &pkginfo.JQSchema{Format: "yaml", Version: ".version", DownloadURL: ".url"},
			Artifact: &pkginfo.Artifact{Type: "rpm", RelBin: "opt/App/app"},
		},
	},
	DesktopName: "App",
}

// vncInfo builds the session info serveDesktop serves, config defaulted like the
// env-parsed one
func vncInfo(guiApp *pkginfo.GUIPkgInfo) *pkginfo.VNCInfo {
	return &pkginfo.VNCInfo{GUIApp: guiApp, Config: &pkginfo.VNCConfig{Resolution: "1600x900"}}
}

// writeChrome bakes one playwright chromium under dir, as `playwright install` does,
// and points the browsers dir env at it
func writeChrome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	chrome := filepath.Join(dir, "chromium-1200", "chrome-linux", "chrome")
	require.NoError(t, os.MkdirAll(filepath.Dir(chrome), artifact.ExecFileMode))
	require.NoError(t, os.WriteFile(chrome, []byte("#!/bin/sh\n"), artifact.ExecFileMode))
	t.Setenv(vncdeps.BrowsersPathEnv, dir)
}

// writeDesktopScript bakes a desktop script recording its $VNC_RESOLUTION to a file,
// points PATH at it, and returns the file's path
func writeDesktopScript(t *testing.T) string {
	t.Helper()
	resolutionFile := filepath.Join(t.TempDir(), "resolution")
	script := filepath.Join(t.TempDir(), "desktop")
	require.NoError(t, os.WriteFile(script,
		[]byte("#!/bin/sh\nprintf '%s' \"$VNC_RESOLUTION\" > "+resolutionFile+"\n"), artifact.ExecFileMode))
	t.Setenv("PATH", filepath.Dir(script))
	return resolutionFile
}

func TestServeDesktop_ErrorsWithoutChromium(t *testing.T) {
	resolutionFile := writeDesktopScript(t)        // PATH finds the desktop script...
	t.Setenv(vncdeps.BrowsersPathEnv, t.TempDir()) // ...but no chromium under the browsers dir

	err := serveDesktop(vncInfo(nil))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no playwright chromium")
	require.Never(t, func() bool {
		_, err := os.ReadFile(resolutionFile) //nolint:gosec // test fixture path
		return err == nil
	}, 100*time.Millisecond, 10*time.Millisecond, "the dependency check must run before the desktop starts")
}

func TestServeDesktop_ErrorsWithoutDesktop(t *testing.T) {
	writeChrome(t)
	t.Setenv("PATH", t.TempDir()) // no desktop script on PATH

	err := serveDesktop(vncInfo(nil))

	require.Error(t, err)
	assert.ErrorIs(t, err, exec.ErrNotFound)
}

func TestServeDesktop_ErrorsWhenInstallFails(t *testing.T) {
	writeChrome(t)
	writeDesktopScript(t)
	testutil.Home(t)

	err := serveDesktop(vncInfo(&rpmInfo))

	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown artifact type "rpm"`)
}

func TestServeDesktop_NoGUIAppServesDesktopAlone(t *testing.T) {
	writeChrome(t)
	resolutionFile := writeDesktopScript(t)

	require.NoError(t, serveDesktop(vncInfo(nil)))
	// the window only bounds the wait: macOS first-spawns freshly-written
	// scripts slowly (Gatekeeper/EDR scans), while success lands in milliseconds
	require.Eventually(t, func() bool {
		body, err := os.ReadFile(resolutionFile) //nolint:gosec // test fixture path
		return err == nil && string(body) == "1600x900"
	}, 5*time.Second, time.Millisecond, "the desktop must start at the run's resolution")
}
