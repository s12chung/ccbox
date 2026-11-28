package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/cfg"
	"github.com/s12chung/ccbox/pkg/dmap"
	"github.com/s12chung/ccbox/pkg/models/guiapp"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

func TestPrintPresentGuardMounts(t *testing.T) {
	setMasksConfig(t)
	var out bytes.Buffer
	restore := log.SwapInfoWriter(&out)
	defer restore()

	printPresentGuardMounts()

	// the absent build stays quiet
	assert.Equal(t, []string{
		"during run, masked with temp filesystem: dist",
		"during run, masked with an empty file, writes discarded: notes.txt",
		"during run, masked with persistent volume: node_modules",
	}, strings.Split(strings.TrimRight(out.String(), "\n"), "\n"))
}

func TestWarnCreatedGuardMounts(t *testing.T) {
	setMasksConfig(t)
	before := append(config.TmpfsMasksPresent(), config.VolumeMasksPresent()...) // prepareRun's snapshot
	var out bytes.Buffer
	restore := log.SwapErrWriter(&out)
	defer restore()

	t.Run("silent while the masks predate the run", func(t *testing.T) {
		warnCreatedGuardMounts(before, nil)
		assert.Empty(t, out.String())
	})

	t.Run("a run's created mask warns", func(t *testing.T) {
		out.Reset()
		warnCreatedGuardMounts([]string{"node_modules"}, nil) // the snapshot predates dist and notes.txt
		assert.Equal(t, "before run, these paths did not exist. future runs will mask them: dist, notes.txt\n", out.String())
	})
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
		dir := t.TempDir()
		body := "cli: opencode@latest\n" +
			"images:\n  base: debian:trixie-20250113-slim\n  mise: ghcr.io/jdx/mise:2026.10.12\n" +
			"vnc:\n  gui_app: " + guiapp.App.Name + "@latest\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".ccbox.yaml"), []byte(body), osutil.File))
		loaded, err := cfg.Load(dir, cfg.Flags{VNC: serveVNC})
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

// setMasksConfig points the config global at a fresh temp project holding tmpfs masks (dist,
// notes.txt), a volume mask (node_modules), and an absent one (build); the global is restored
// after the test.
func setMasksConfig(t *testing.T) {
	t.Helper()
	testutil.Home(t)
	dir := t.TempDir()
	body := "cli: pi@latest\n" +
		"images:\n  base: debian:trixie-20250113-slim\n  mise: ghcr.io/jdx/mise:2026.10.12\n" +
		"tmpfs_masks:\n  - dist\n  - notes.txt\n  - build\n" +
		"volume_masks:\n  - node_modules\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".ccbox.yaml"), []byte(body), osutil.File))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "dist"), osutil.Dir))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "node_modules"), osutil.Dir))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), nil, osutil.File))
	loaded, err := cfg.Load(dir, cfg.Flags{})
	require.NoError(t, err)
	saved := config
	t.Cleanup(func() { config = saved })
	config = loaded
}
