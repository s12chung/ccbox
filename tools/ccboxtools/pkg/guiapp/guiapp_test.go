package guiapp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/fsutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

// debInfo is a minimal valid deb-sourced GUI app, like the image's real one.
var debInfo = pkginfo.GUIPkgInfo{
	PkgInfo: pkginfo.PkgInfo{
		Name: "app",
		ReleaseURL: &pkginfo.ReleaseURL{
			URL:      "https://example.dev/manifest",
			JQSchema: &pkginfo.JQSchema{Format: "yaml", Version: ".version", DownloadURL: ".url"},
			Artifact: &pkginfo.Artifact{Type: "deb", RelBin: "opt/App/app"},
		},
	},
	Args:        []string{"--no-sandbox"},
	DesktopName: "App",
}

// mustVNCJSON renders g as the run's VNCConfigEnvVar JSON.
func mustVNCJSON(t *testing.T, g pkginfo.GUIPkgInfo) string {
	t.Helper()
	body, err := json.Marshal(pkginfo.VNCInfo{GUIApp: &g, Config: &pkginfo.VNCConfig{Resolution: "1600x900"}})
	require.NoError(t, err)
	return string(body)
}

// load sets body as the run's VNC_CONFIG and returns the parsed session info
func load(t *testing.T, body string) *pkginfo.VNCInfo {
	t.Helper()
	t.Setenv(pkginfo.VNCConfigEnvVar, body)
	return Load()
}

func TestLoad(t *testing.T) {
	t.Run("unset env reads as no vnc", func(t *testing.T) {
		assert.Nil(t, load(t, ""))
	})

	t.Run("parses the body", func(t *testing.T) {
		info := load(t, mustVNCJSON(t, debInfo))

		require.NotNil(t, info)
		require.NotNil(t, info.GUIApp)
		assert.Equal(t, debInfo, *info.GUIApp)
		assert.Equal(t, "1600x900", info.Config.Resolution)
	})

	t.Run("a bad body warns and reads as no vnc", func(t *testing.T) {
		assert.Nil(t, load(t, "{"))
	})
}

func TestBinFor(t *testing.T) {
	bin, err := binFor(debInfo)

	require.NoError(t, err)
	assert.Equal(t, "/opt/ccbox/apps/app/current/opt/App/app", bin)
}

func TestInstall_Errors(t *testing.T) {
	// a HOME that can't hold the menu entry fails the install, erroring the boot
	home := filepath.Join(t.TempDir(), "home")
	require.NoError(t, os.WriteFile(home, nil, fileMode)) // a file, not a dir
	t.Setenv("HOME", home)

	err := Install(debInfo)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "write app menu entry")
}

func TestWriteDesktopEntry(t *testing.T) {
	t.Run("writes branded entry", func(t *testing.T) {
		home := testutil.Home(t)

		require.NoError(t, writeDesktopEntry(debInfo))

		path := filepath.Join(home, ".local", "share", "applications", "app.desktop")
		body, err := os.ReadFile(path) //nolint:gosec // test fixture path
		require.NoError(t, err)
		assert.Contains(t, string(body), "Name=App")
		assert.Contains(t, string(body), "Exec=/usr/local/bin/ccboxtools guiapp exec %U")
	})

	t.Run("overwrites stale entry", func(t *testing.T) {
		home := testutil.Home(t)
		path := filepath.Join(home, ".local", "share", "applications", "app.desktop")
		require.NoError(t, os.MkdirAll(filepath.Dir(path), fsutil.DirMode))
		require.NoError(t, os.WriteFile(path, []byte("stale"), fileMode))

		require.NoError(t, writeDesktopEntry(debInfo))

		body, err := os.ReadFile(path) //nolint:gosec // test fixture path
		require.NoError(t, err)
		assert.NotEqual(t, "stale", string(body))
	})

	t.Run("no desktop name, skips", func(t *testing.T) {
		home := testutil.Home(t)
		info := debInfo
		info.DesktopName = ""

		require.NoError(t, writeDesktopEntry(info))

		_, err := os.Stat(filepath.Join(home, ".local"))
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestExec_NotInstalled(t *testing.T) {
	err := Exec(debInfo, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not installed at /opt/ccbox/apps/app/current/opt/App/app")
}

func TestInstalled_ExecBit(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name  string
		mode  os.FileMode
		want  bool
		exist bool
	}{
		{"missing", 0, false, false},
		{"no exec bit", fileMode, false, true},
		{"exec bit", fileMode | 0o100, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bin := filepath.Join(dir, tt.name)
			if tt.exist {
				require.NoError(t, os.WriteFile(bin, nil, tt.mode))
			}

			assert.Equal(t, tt.want, installed(bin))
		})
	}
}

func TestWaitInstalled(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "app")
	sleeps := 0

	waitInstalled(bin, func() {
		sleeps++
		require.NoError(t, os.WriteFile(bin, nil, fileMode|0o100))
	})

	assert.Equal(t, 1, sleeps)
}
