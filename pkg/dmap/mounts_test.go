package dmap

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/cfg"
	"github.com/s12chung/ccbox/pkg/dmap/share"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/models/cli"
	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/slug"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

func TestTmpfsMasks(t *testing.T) {
	assert.Equal(t, []string{"/home/ccbox/proj/.idea", "/home/ccbox/proj/dist"},
		tmpfsMasks("/Users/me/proj", []string{".idea", "dist"}))
}

// testRunMap builds a RunMap over a fresh temp home and a fresh temp project's config.
func testRunMap(t *testing.T, flags cfg.Config) *RunMap {
	t.Helper()
	testutil.Home(t)
	if flags.CLIName == nil {
		flags.CLIName = new("codex")
	}
	config, err := cfg.Load(t.TempDir(), flags, false)
	require.NoError(t, err)
	return NewRunMap(t.TempDir(), config, false)
}

func TestProxyAllowBinds(t *testing.T) {
	bind := docker.NewBind(proxyLiveDir(), pkginfo.ProxyMount).ReadOnly()

	assert.Equal(t, []docker.Mount{bind}, proxyAllowBinds(false))
	assert.Empty(t, proxyAllowBinds(true))
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

		config, err := cfg.Load(t.TempDir(), cfg.Config{
			CLIName:       new("codex"),
			ReadOnlyBinds: map[string]string{cfg.GitConfigKey: firmrule.EnabledValue},
		}, false)
		require.NoError(t, err)

		assert.Equal(t, []docker.Mount{
			docker.NewBind(filepath.Join(home, ".config", "git"), cfg.GitConfigMount).ReadOnly(),
		}, readOnlyBinds(config.ReadOnlyBindsPresent()))
	})

	t.Run("no bind when the host git dir is absent", func(t *testing.T) {
		testutil.Home(t)

		config, err := cfg.Load(t.TempDir(), cfg.Config{
			CLIName:       new("codex"),
			ReadOnlyBinds: map[string]string{cfg.GitConfigKey: firmrule.EnabledValue},
		}, false)
		require.NoError(t, err)

		assert.Empty(t, readOnlyBinds(config.ReadOnlyBindsPresent()))
	})
}

func TestCLIDataBinds(t *testing.T) {
	t.Run("renders rw binds under the container home, sorted for a deterministic spec", func(t *testing.T) {
		userDir := "/home/me/.ccbox"
		c := cli.CLI{PkgInfo: pkginfo.PkgInfo{Name: "opencode"}, DataBinds: map[string]*string{
			".local/share/opencode/auth.json":     new("{}"), // file with seed content
			".local/share/opencode/sessions.json": nil,       // dir despite the extension
			".config/opencode":                    nil,       // dir
		}}

		assert.Equal(t, []docker.Mount{
			docker.NewBind(filepath.Join(userDir, "data", "opencode", ".config-opencode"), path.Join(cfg.ContainerHome, ".config/opencode")),
			docker.NewBind(
				filepath.Join(userDir, "data", "opencode", ".local-share-opencode-auth.json"),
				path.Join(cfg.ContainerHome, ".local/share/opencode/auth.json"),
			),
			docker.NewBind(
				filepath.Join(userDir, "data", "opencode", ".local-share-opencode-sessions.json"),
				path.Join(cfg.ContainerHome, ".local/share/opencode/sessions.json"),
			),
		}, cliDataBinds(userDir, c))
	})

	t.Run("no binds when the CLI has none", func(t *testing.T) {
		assert.Empty(t, cliDataBinds("/home/me/.ccbox", cli.CLI{PkgInfo: pkginfo.PkgInfo{Name: "mycli"}}))
	})
}

func TestCliScratchBind(t *testing.T) {
	t.Run("binds the scratch file at the agents scratch mount", func(t *testing.T) {
		for _, cliName := range []string{"claude", "codex", "opencode"} {
			scratchFile := "/host/.ccbox/tmp/agents/" + cliName + "/AGENTS.md"
			assert.Equal(t,
				[]docker.Mount{docker.NewBind(scratchFile, share.AgentsMdScratchMount(cliName))},
				cliScratchBind(scratchFile, share.AgentsMdScratchMount(cliName)), cliName)
		}
	})

	t.Run("no bind when scratchFile is empty (no scratch written)", func(t *testing.T) {
		assert.Empty(t, cliScratchBind("", ""))
	})
}

func TestCacheVolumesMap(t *testing.T) {
	// today's literal names: existing host volumes keep matching
	assert.Equal(t, map[string]string{
		"cache":      "/home/ccbox/.cache",
		"gem":        "/home/ccbox/.gem",
		"go":         "/home/ccbox/go",
		"local":      "/home/ccbox/.local",
		"npm":        "/home/ccbox/.npm",
		"npm-global": "/home/ccbox/.npm-global",
		"tmp":        "/tmp",
	}, cacheVolumesMap)

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

func TestCacheVolumeNames(t *testing.T) {
	s := slug.Path("/Users/me/proj")
	assert.Equal(t, map[string]string{
		"ccbox" + s + "-cache-cache-default":       "/home/ccbox/.cache",
		"ccbox" + s + "-gem-cache-default":         "/home/ccbox/.gem",
		"ccbox" + s + "-go-cache-default":          "/home/ccbox/go",
		"ccbox" + s + "-local-cache-default":       "/home/ccbox/.local",
		"ccbox" + s + "-npm-cache-default":         "/home/ccbox/.npm",
		"ccbox" + s + "-npm--global-cache-default": "/home/ccbox/.npm-global",
		"ccbox" + s + "-tmp-cache-default":         "/tmp",
	}, cacheVolumeNames("/Users/me/proj"))

	// protect against mounts at "tmp"
	assert.NotContains(t, cacheVolumeNames("/Users/me/proj"), volumeName("/Users/me/proj", "tmp"))
}

func TestVolumeName(t *testing.T) {
	assert.Equal(t, "ccbox-Users-me-proj-go", volumeName("/Users/me/proj", "go"))
	assert.Equal(t, "ccbox-Users-me-proj-go-with-me", volumeName("/Users/me/proj", "go/with/me"))
	assert.Equal(t, "ccbox-Users-me-my--proj-go", volumeName("/Users/me/my-proj", "go"))
}
