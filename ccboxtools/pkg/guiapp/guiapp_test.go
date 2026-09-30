package guiapp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/ccboxtools/pkg/util/fsutil"
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

// mustJSON renders g as the GUIAppEnvVar JSON.
func mustJSON(t *testing.T, g pkginfo.GUIPkgInfo) string {
	t.Helper()
	body, err := json.Marshal(g)
	require.NoError(t, err)
	return string(body)
}

func TestInstallGUIApp(t *testing.T) {
	t.Run("no env, skips", func(t *testing.T) {
		t.Setenv(pkginfo.GUIAppEnvVar, "")
		Install()
	})

	t.Run("bad json warns, not fails", func(t *testing.T) {
		t.Setenv(pkginfo.GUIAppEnvVar, "{")
		Install()
	})
}

func TestWriteDesktopEntry(t *testing.T) {
	t.Run("writes branded entry", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		require.NoError(t, writeDesktopEntry(debInfo))

		path := filepath.Join(home, ".local", "share", "applications", "app.desktop")
		body, err := os.ReadFile(path) //nolint:gosec // test fixture path
		require.NoError(t, err)
		assert.Contains(t, string(body), "Name=App")
		assert.Contains(t, string(body), "Exec=/usr/local/bin/ccboxtools guiapp exec %U")
	})

	t.Run("overwrites stale entry", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		path := filepath.Join(home, ".local", "share", "applications", "app.desktop")
		require.NoError(t, os.MkdirAll(filepath.Dir(path), fsutil.DirMode))
		require.NoError(t, os.WriteFile(path, []byte("stale"), fileMode))

		require.NoError(t, writeDesktopEntry(debInfo))

		body, err := os.ReadFile(path) //nolint:gosec // test fixture path
		require.NoError(t, err)
		assert.NotEqual(t, "stale", string(body))
	})

	t.Run("no desktop name, skips", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		info := debInfo
		info.DesktopName = ""

		require.NoError(t, writeDesktopEntry(info))

		_, err := os.Stat(filepath.Join(home, ".local"))
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestResolve(t *testing.T) {
	t.Run("unset env", func(t *testing.T) {
		t.Setenv(pkginfo.GUIAppEnvVar, "")

		_, _, err := resolve()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "GUIAPP_PKGINFO is not set")
	})

	t.Run("joins apps root path", func(t *testing.T) {
		body := mustJSON(t, debInfo)
		t.Setenv(pkginfo.GUIAppEnvVar, body)

		info, bin, err := resolve()

		require.NoError(t, err)
		assert.Equal(t, debInfo, info)
		assert.Equal(t, "/opt/ccbox/apps/app/current/opt/App/app", bin)
	})
}

func TestExec_NotInstalled(t *testing.T) {
	t.Setenv(pkginfo.GUIAppEnvVar, mustJSON(t, debInfo))

	err := Exec(nil)

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
