package vnc

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkger/artifact"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/vnc/vncdeps"
)

// failingGUIApp is Go-constructed only: the config's validation rejects the artifact
// type, so the install fails before any download
var failingGUIApp = &pkginfo.GUIPkgInfo{
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

// writeDesktopScript bakes a desktop script marking a file as started, points PATH
// at it, and returns the file's path
func writeDesktopScript(t *testing.T) string {
	t.Helper()
	desktopFile := filepath.Join(t.TempDir(), "desktop-started")
	script := filepath.Join(t.TempDir(), "desktop")
	require.NoError(t, os.WriteFile(script,
		[]byte("#!/bin/sh\n: > "+desktopFile+"\n"), artifact.ExecFileMode))
	t.Setenv("PATH", filepath.Dir(script))
	return desktopFile
}

func TestServe_ErrorsWithoutChromium(t *testing.T) {
	desktopFile := writeDesktopScript(t)           // PATH finds the desktop script...
	t.Setenv(vncdeps.BrowsersPathEnv, t.TempDir()) // ...but no chromium under the browsers dir
	t.Setenv("http_proxy", "")                     // the no-proxy landing

	svcs, err := Serve(&pkginfo.VNCInfo{})

	require.Error(t, err)
	assert.Nil(t, svcs)
	assert.Contains(t, err.Error(), "no playwright chromium")
	require.Never(t, func() bool {
		_, err := os.Stat(desktopFile) //nolint:gosec // test fixture path
		return err == nil
	}, 100*time.Millisecond, 10*time.Millisecond, "the dependency check must run before the desktop starts")
}

func TestServe_ProxiedErrorsWithoutChromium(t *testing.T) {
	// the dependency check gates the room's serve too — no process forks first
	t.Setenv("http_proxy", "http://"+pkginfo.ProxyAddr+":"+strconv.Itoa(pkginfo.ProxyPort)) // the proxied landing
	t.Setenv(vncdeps.BrowsersPathEnv, t.TempDir())                                          // no chromium under the browsers dir

	svcs, err := Serve(&pkginfo.VNCInfo{})

	require.Error(t, err)
	assert.Nil(t, svcs)
	assert.Contains(t, err.Error(), "no playwright chromium")
}

func TestDesktopSession_Ready_ReturnsOnPing(t *testing.T) {
	session := &desktopSession{}
	cmd, err := session.CmdToStart(t.Context())
	require.NoError(t, err)
	require.Len(t, cmd.ExtraFiles, 1, "the fork carries the ready pipe")
	_, err = cmd.ExtraFiles[0].Write([]byte{'x'}) // the session's landing ping
	require.NoError(t, err)

	require.NoError(t, session.Ready(t.Context(), cmd))
}

func TestDesktopSession_Ready_EOFsOnDeath(t *testing.T) {
	session := &desktopSession{}
	cmd, err := session.CmdToStart(t.Context())
	require.NoError(t, err)

	err = session.Ready(t.Context(), cmd) // no ping lands: the child died before sending it

	require.Error(t, err)
	assert.Contains(t, err.Error(), "child exited before going ready")
}

func TestSetupDesktop_ErrorsWhenInstallFails(t *testing.T) {
	// the install precedes the exec — a failing download can't leave a bare session
	t.Setenv("HOME", t.TempDir()) // writeDesktopEntry aims at the home

	_, err := SetupDesktop(t.Context(), &pkginfo.VNCInfo{GUIApp: failingGUIApp})

	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown artifact type "rpm"`)
}
