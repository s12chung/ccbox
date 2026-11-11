package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/cfg"
	"github.com/s12chung/ccbox/pkg/dmap"
	"github.com/s12chung/ccbox/pkg/models/guiapp"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

func TestResumeArgs(t *testing.T) {
	defer func() { execMode.Resume = false }()

	tests := []struct {
		name    string
		resume  bool
		args    []string
		wantErr bool
	}{
		{name: "no args is fine", resume: false, args: nil},
		{name: "name with -r is fine", resume: true, args: []string{"auth-refactor"}},
		{name: "name without -r is rejected", resume: false, args: []string{"auth-refactor"}, wantErr: true},
		{name: "two names is rejected", resume: true, args: []string{"a", "b"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execMode.Resume = tt.resume
			err := rootCmd.Args(nil, tt.args)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSeedDataBinds(t *testing.T) {
	t.Run("seeds the file under its data dir", func(t *testing.T) {
		userDir := t.TempDir()
		dataDir := dmap.CLIDataDir(userDir, "opencode")

		require.NoError(t, seedDataBinds(dataDir, map[string]*string{".local-share-opencode-auth.json": new("{}")}, osutil.File))

		got, err := os.ReadFile(filepath.Join(dataDir, ".local-share-opencode-auth.json")) // #nosec G304 -- the test's own seeded path
		require.NoError(t, err)
		assert.Equal(t, "{}", string(got))
	})

	t.Run("creates the dir bind", func(t *testing.T) {
		userDir := t.TempDir()
		dataDir := dmap.CLIDataDir(userDir, "mycli")

		require.NoError(t, seedDataBinds(dataDir, map[string]*string{".local-share-mycli-store": nil}, osutil.File))

		info, err := os.Stat(filepath.Join(dataDir, ".local-share-mycli-store"))
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("seeds the file's content chmod'd to fileMode", func(t *testing.T) {
		userDir := t.TempDir()
		dataDir := dmap.GUIAppDataDir(userDir, "zcode", false)

		require.NoError(t, seedDataBinds(dataDir, map[string]*string{".zcode-v2/credential-secret": new("sec")}, osutil.SecretFile))

		host := filepath.Join(dataDir, ".zcode-v2", "credential-secret")
		got, err := os.ReadFile(host) // #nosec G304 -- the test's own seeded path
		require.NoError(t, err)
		assert.Equal(t, "sec", string(got))

		info, err := os.Stat(host)
		require.NoError(t, err)
		assert.Equal(t, osutil.SecretFile, info.Mode().Perm())
	})

	t.Run("skips an existing file", func(t *testing.T) {
		userDir := t.TempDir()
		dataDir := dmap.CLIDataDir(userDir, "opencode")
		host := filepath.Join(dataDir, ".local-share-opencode-auth.json")
		require.NoError(t, os.MkdirAll(filepath.Dir(host), osutil.Dir))
		require.NoError(t, os.WriteFile(host, []byte(`{"real":"creds"}`), osutil.File))

		require.NoError(t, seedDataBinds(dataDir, map[string]*string{".local-share-opencode-auth.json": new("{}")}, osutil.File))

		got, err := os.ReadFile(host) // #nosec G304 -- the test's own seeded path
		require.NoError(t, err)
		assert.JSONEq(t, `{"real":"creds"}`, string(got), "the existing file — real creds — is never clobbered")
	})
}

func TestSeedRunMounts(t *testing.T) {
	// setConfig swaps the config global, restoring it after; testutil.Home keeps
	// the cli config seed off the real home
	setConfig := func(t *testing.T, serveVNC bool) {
		t.Helper()
		testutil.Home(t)
		loaded, err := cfg.Load(t.TempDir(), cfg.Config{
			CLIName: new("opencode"),
			VNC:     &cfg.VNC{GUIAppName: guiapp.App.Name},
		}, serveVNC)
		require.NoError(t, err)
		saved := config
		t.Cleanup(func() { config = saved })
		config = loaded
	}

	t.Run("a vnc run seeds the cli's and gui app's binds", func(t *testing.T) {
		setConfig(t, true)
		userDir := t.TempDir()

		require.NoError(t, seedRunMounts(userDir, false))
		assert.FileExists(t, filepath.Join(dmap.CLIDataDir(userDir, "opencode"), ".local-share-opencode-auth.json"))
		assert.FileExists(t, filepath.Join(dmap.GUIAppDataDir(userDir, guiapp.App.Name, false), ".zcode-v2", "credential-secret"))
	})

	t.Run("a headless run seeds no gui app binds", func(t *testing.T) {
		setConfig(t, false)
		userDir := t.TempDir()

		require.NoError(t, seedRunMounts(userDir, true))
		assert.NoFileExists(t, dmap.GUIAppDataDir(userDir, guiapp.App.Name, true))
	})

	t.Run("a vnc run without a gui app seeds no app binds", func(t *testing.T) {
		setConfig(t, true)
		config.VNC = &cfg.VNC{}
		userDir := t.TempDir()

		require.NoError(t, seedRunMounts(userDir, false))
		assert.NoFileExists(t, dmap.GUIAppDataDir(userDir, guiapp.App.Name, false))
	})
}
