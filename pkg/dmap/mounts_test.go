package dmap

import (
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/cfg"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/mise"
	"github.com/s12chung/ccbox/pkg/models/app"
	"github.com/s12chung/ccbox/pkg/models/cli"
	"github.com/s12chung/ccbox/pkg/models/guiapp"
	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/klean"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/slug"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

func TestTmpfsMasks(t *testing.T) {
	assert.Equal(t, []string{"/home/ccbox/proj/.idea", "/home/ccbox/proj/dist"},
		tmpfsMasks("/Users/me/proj", []string{".idea", "dist"}))
}

// writeConfigYaml writes body as the project's .ccbox.yaml in a fresh temp project,
// returning the project dir
func writeConfigYaml(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	cli, rest, found := strings.Cut(body, "\n")
	require.Truef(t, found && strings.HasPrefix(cli, "cli:"), "body must lead with the cli line: %q", body)
	body = cli + "\n" +
		"images:\n  base: debian:trixie-20250113-slim\n  mise: ghcr.io/jdx/mise:2026.10.12\n" +
		rest
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".ccbox.yaml"), []byte(body), osutil.File))
	return dir
}

// loadConfig loads cfg over a fresh temp project whose .ccbox.yaml carries body —
// testutil.Home keeps the user-level layer out of the way
func loadConfig(t *testing.T, body string) *cfg.Config {
	t.Helper()
	testutil.Home(t)
	config, err := cfg.Load(writeConfigYaml(t, body), cfg.Flags{})
	require.NoError(t, err)
	return config
}

// testRunMap builds a RunMap over a fresh temp project whose .ccbox.yaml carries body;
// the cli defaults to codex when body doesn't name one.
func testRunMap(t *testing.T, body string) *RunMap {
	t.Helper()
	if !strings.Contains(body, "cli:") {
		body = "cli: codex\n" + body
	}
	return NewRunMap(loadConfig(t, body), runtime.AllCacheVolumes(), false)
}

func TestProxyAllowBind(t *testing.T) {
	bind := docker.NewBind(proxyLiveDir(), pkginfo.ProxyMount).ReadOnly()

	assert.Equal(t, []docker.Mount{bind}, proxyAllowBind(true))
	assert.Empty(t, proxyAllowBind(false))
}

func TestVolumes(t *testing.T) {
	// sorted for a deterministic spec; global marks a volume shared by every project
	assert.Equal(t, []docker.Mount{
		docker.NewVolume("ccbox-a", "/a").Global(),
		docker.NewVolume("ccbox-b", "/b").Global(),
	}, volumes(map[string]string{"ccbox-b": "/b", "ccbox-a": "/a"}, true))

	assert.Equal(t, []docker.Mount{docker.NewVolume("ccbox-a", "/a")},
		volumes(map[string]string{"ccbox-a": "/a"}, false))

	assert.Empty(t, volumes(nil, true))
}

func TestVolumeMasks(t *testing.T) {
	// the name slugifies the mask path under the project slug; ensured fresh, then
	// chowned to the container user at run start
	assert.Equal(t, []docker.Mount{
		docker.NewVolume("ccbox-Users-me-proj-node_modules", "/home/ccbox/proj/node_modules"),
		docker.NewVolume("ccbox-Users-me-proj-vendor-bundle", "/home/ccbox/proj/vendor/bundle"),
	}, volumeMasks("/Users/me/proj", []string{"node_modules", "vendor/bundle"}))
}

func TestReadOnlyGlobBinds(t *testing.T) {
	assert.Equal(t, []docker.Mount{
		docker.NewBind("/Users/me/proj/.env", "/home/ccbox/proj/.env").ReadOnly(),
		docker.NewBind("/Users/me/proj/certs/server.pem", "/home/ccbox/proj/certs/server.pem").ReadOnly(),
	}, readOnlyGlobBinds("/Users/me/proj", []string{".env", "certs/server.pem"}))
}

func TestReadOnlyBinds(t *testing.T) {
	t.Run("no binds without entries", func(t *testing.T) {
		assert.Empty(t, readOnlyBinds(nil))
	})

	t.Run("binds each pair read-only, sorted by the container mount", func(t *testing.T) {
		assert.Equal(t, []docker.Mount{
			docker.NewBind("/srv/ca", "/home/ccbox/.local/share/ca").ReadOnly(),
			docker.NewBind("/home/me/fonts", "/home/ccbox/fonts").ReadOnly(),
		}, readOnlyBinds(map[string]string{
			"/home/me/fonts": "/home/ccbox/fonts", // unsorted input
			"/srv/ca":        "/home/ccbox/.local/share/ca",
		}))
	})

	t.Run("binds the enabled gitconfig at the container's default XDG path", func(t *testing.T) {
		home := testutil.Home(t)
		require.NoError(t, os.MkdirAll(filepath.Join(home, ".config", "git"), osutil.Dir))

		config, err := cfg.Load(writeConfigYaml(t, "cli: codex\nread_only_binds:\n  gitconfig: enabled\n"), cfg.Flags{})
		require.NoError(t, err)

		assert.Equal(t, []docker.Mount{
			docker.NewBind(filepath.Join(home, ".config", "git"), cfg.GitConfigMount).ReadOnly(),
		}, readOnlyBinds(config.ReadOnlyBindsPresent()))
	})

	t.Run("no bind when the host git dir is absent", func(t *testing.T) {
		testutil.Home(t)

		config, err := cfg.Load(writeConfigYaml(t, "cli: codex\nread_only_binds:\n  gitconfig: enabled\n"), cfg.Flags{})
		require.NoError(t, err)

		assert.Empty(t, readOnlyBinds(config.ReadOnlyBindsPresent()))
	})
}

func TestDataBinds(t *testing.T) {
	t.Run("renders rw binds under the cli's data dir, sorted for a deterministic spec", func(t *testing.T) {
		userDir := "/home/me/.ccbox"
		c := cli.CLI{PkgInfo: pkginfo.PkgInfo{Name: "opencode"}, Setup: app.Setup{DataBinds: map[string]*string{
			".local/share/opencode/auth.json":     new("{}"), // file with seed content
			".local/share/opencode/sessions.json": nil,       // dir despite the extension
			".config/opencode":                    nil,       // dir
		}}}

		assert.Equal(t, []docker.Mount{
			docker.NewBind(filepath.Join(CLIDataDir(userDir, "opencode"), ".config-opencode"), path.Join(pkginfo.ContainerHome, ".config/opencode")),
			docker.NewBind(filepath.Join(CLIDataDir(userDir, "opencode"), ".local-share-opencode-auth.json"),
				path.Join(pkginfo.ContainerHome, ".local/share/opencode/auth.json")),
			docker.NewBind(filepath.Join(CLIDataDir(userDir, "opencode"), ".local-share-opencode-sessions.json"),
				path.Join(pkginfo.ContainerHome, ".local/share/opencode/sessions.json")),
		}, dataBinds(CLIDataDir(userDir, c.Name), c.Setup.DataBindMounts(false)))
	})

	t.Run("no binds when the app has none", func(t *testing.T) {
		assert.Empty(t, dataBinds(CLIDataDir("/home/me/.ccbox", "mycli"), app.Setup{}.DataBindMounts(false)))
	})
}

func TestRunMap_GUIAppDataBinds(t *testing.T) {
	// newVncRunMap maps a vnc run naming guiApp
	newVncRunMap := func(t *testing.T, guiApp string) *RunMap {
		t.Helper()
		body := "cli: claude\n"
		if guiApp != "" {
			body += "vnc:\n  gui_app: " + guiApp + "\n"
		}
		config, err := cfg.Load(writeConfigYaml(t, body), cfg.Flags{VNC: true})
		require.NoError(t, err)
		return NewRunMap(config, nil, false)
	}

	t.Run("renders the gui app's binds like the cli's — the bound dir carries its seeds", func(t *testing.T) {
		binds := newVncRunMap(t, guiapp.App.Name).guiAppDataBinds(true)
		assert.Equal(t, []docker.Mount{
			docker.NewBind(
				filepath.Join(GUIAppDataDir(userdir.Dir(), "zcode", true), ".ccbox-xdg-share-applications-zcode.desktop"),
				path.Join(pkginfo.ContainerHome, ".ccbox/xdg/share/applications/zcode.desktop"),
			),
			docker.NewBind(
				filepath.Join(GUIAppDataDir(userdir.Dir(), "zcode", true), ".zcode-v2"),
				path.Join(pkginfo.ContainerHome, ".zcode/v2"),
			),
		}, binds, "the app's atomic rewrites rename within the bound dir")
	})

	t.Run("the plain run mounts the no-proxy tree's plain binds", func(t *testing.T) {
		binds := newVncRunMap(t, guiapp.App.Name).guiAppDataBinds(false)
		assert.Equal(t, []docker.Mount{
			docker.NewBind(
				filepath.Join(GUIAppDataDir(userdir.Dir(), "zcode", false), ".ccbox-xdg-share-applications-zcode.desktop"),
				path.Join(pkginfo.ContainerHome, ".ccbox/xdg/share/applications/zcode.desktop"),
			),
			docker.NewBind(
				filepath.Join(GUIAppDataDir(userdir.Dir(), "zcode", false), ".zcode-v2"),
				path.Join(pkginfo.ContainerHome, ".zcode/v2"),
			),
		}, binds)
	})

	t.Run("none without a vnc run", func(t *testing.T) {
		config := loadConfig(t, "cli: claude\nvnc:\n  gui_app: "+guiapp.App.Name+"\n")

		binds := NewRunMap(config, nil, false).guiAppDataBinds(true)
		assert.Empty(t, binds)
	})

	t.Run("none without a gui app", func(t *testing.T) {
		binds := newVncRunMap(t, "").guiAppDataBinds(true)
		assert.Empty(t, binds)
	})
}

func TestShareBinds(t *testing.T) {
	t.Run("binds the share at its container mount", func(t *testing.T) {
		bind := klean.ShareBind{HostPath: "/host/.ccbox/skills", ContainerPath: "/home/ccbox/.claude/skills"}
		assert.Equal(t,
			[]docker.Mount{docker.NewBind(bind.HostPath, bind.ContainerPath)},
			shareBinds(bind))
	})

	t.Run("no bind for the zero bind — the share sat out the run", func(t *testing.T) {
		assert.Empty(t, shareBinds(klean.ShareBind{}))
	})
}

func TestCacheVolumeNames(t *testing.T) {
	s := slug.Path("/Users/me/proj")
	assert.Equal(t, map[string]string{
		"ccbox" + s + "-cache-cache-default":       "/home/ccbox/.cache",
		"ccbox" + s + "-cargo-cache-default":       "/home/ccbox/.cargo",
		"ccbox" + s + "-go-cache-default":          "/home/ccbox/go",
		"ccbox" + s + "-local-cache-default":       "/home/ccbox/.local",
		"ccbox" + s + "-npm-cache-default":         "/home/ccbox/.npm",
		"ccbox" + s + "-npm--global-cache-default": "/home/ccbox/.npm-global",
		"ccbox" + s + "-tmp-cache-default":         "/tmp",
	}, cacheVolumeNames("/Users/me/proj", runtime.AllCacheVolumes()))

	// protect against mounts at "tmp"
	assert.NotContains(t, cacheVolumeNames("/Users/me/proj", runtime.AllCacheVolumes()), volumeName("/Users/me/proj", "tmp"))

	// a colliding suffix silently overwrites: every cache volume composes in
	suffixes := map[string]bool{}
	for _, r := range runtime.All() {
		for _, dir := range r.CacheVolumes {
			suffix := strings.TrimPrefix(path.Base(dir), ".")
			assert.Falsef(t, suffixes[suffix], "suffix %q collides", suffix)
			suffixes[suffix] = true
		}
	}
}

// TestCacheVolumeNames_Selection pins the per-project volumes following the
// mise config's selection.
func TestCacheVolumeNames_Selection(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(mise.ProjectConfigPath(dir), []byte("[tools]\ngo = \"1.26\"\n"), osutil.File))
	selected, err := runtime.FromMiseConfig(dir)
	require.NoError(t, err)

	names := cacheVolumeNames("/Users/me/proj", selected.CacheVolumes())
	assert.ElementsMatch(t, slices.Collect(maps.Keys(names)), []string{
		"ccbox" + slug.Path("/Users/me/proj") + "-cache-cache-default", // sharedCacheVolumes, in every run
		"ccbox" + slug.Path("/Users/me/proj") + "-local-cache-default",
		"ccbox" + slug.Path("/Users/me/proj") + "-tmp-cache-default",
		"ccbox" + slug.Path("/Users/me/proj") + "-go-cache-default",
		"ccbox" + slug.Path("/Users/me/proj") + "-npm-cache-default",
		"ccbox" + slug.Path("/Users/me/proj") + "-npm--global-cache-default",
	})
}

func TestVolumeName(t *testing.T) {
	assert.Equal(t, "ccbox-Users-me-proj-go", volumeName("/Users/me/proj", "go"))
	assert.Equal(t, "ccbox-Users-me-proj-go-with-me", volumeName("/Users/me/proj", "go/with/me"))
	assert.Equal(t, "ccbox-Users-me-my--proj-go", volumeName("/Users/me/my-proj", "go"))
}
