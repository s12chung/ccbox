package dmap

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/slug"
)

func TestTmpfsMasks(t *testing.T) {
	assert.Equal(t, []string{"/home/ccbox/proj/.idea", "/home/ccbox/proj/dist"},
		tmpfsMasks("/Users/me/proj", []string{".idea", "dist"}))
}

// testRunMap builds a RunMap over a fresh temp project's config.
func testRunMap(t *testing.T, flags projectcfg.Config) *RunMap {
	t.Helper()
	if flags.CLIName == nil {
		flags.CLIName = new("codex")
	}
	flags.HostGitConfig = new(false)
	cfg, err := projectcfg.Load(t.TempDir(), flags)
	require.NoError(t, err)
	return NewRunMap(t.TempDir(), cfg)
}

func TestVolumeMasks(t *testing.T) {
	// the name slugifies the mask path under the project slug; owned so the run
	// seeds the volume with its own content
	assert.Equal(t, []docker.Mount{
		docker.NewVolume("ccbox-Users-me-proj-node_modules", "/home/ccbox/proj/node_modules").Owned(),
		docker.NewVolume("ccbox-Users-me-proj-vendor-bundle", "/home/ccbox/proj/vendor/bundle").Owned(),
	}, volumeMasks("/Users/me/proj", []string{"node_modules", "vendor/bundle"}))
}

func TestReadOnlyBinds(t *testing.T) {
	assert.Equal(t, []docker.Mount{
		docker.NewBind("/Users/me/proj/.env", "/home/ccbox/proj/.env").ReadOnly(),
		docker.NewBind("/Users/me/proj/certs/server.pem", "/home/ccbox/proj/certs/server.pem").ReadOnly(),
	}, readOnlyBinds("/Users/me/proj", []string{".env", "certs/server.pem"}))
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

func TestGitBinds(t *testing.T) {
	t.Run("no bind when disabled", func(t *testing.T) {
		assert.Nil(t, gitBinds(false))
	})

	t.Run("no bind when the host git dir is absent", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		t.Setenv("XDG_CONFIG_HOME", "")
		assert.Nil(t, gitBinds(true))
	})

	t.Run("binds the host git dir read-only at git's XDG path", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", "")
		require.NoError(t, os.MkdirAll(filepath.Join(home, ".config", "git"), ioutil.Dir))

		assert.Equal(t, []docker.Mount{
			docker.NewBind(filepath.Join(home, ".config", "git"), gitConfigMount).ReadOnly(),
		}, gitBinds(true))
	})

	t.Run("binds XDG_CONFIG_HOME's git dir at the container's default XDG path", func(t *testing.T) {
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)
		require.NoError(t, os.MkdirAll(filepath.Join(xdg, "git"), ioutil.Dir))

		assert.Equal(t, []docker.Mount{
			docker.NewBind(filepath.Join(xdg, "git"), gitConfigMount).ReadOnly(),
		}, gitBinds(true))
	})
}

func TestCLIDataBinds(t *testing.T) {
	t.Run("renders rw binds under containerHome, sorted for a deterministic spec", func(t *testing.T) {
		userDir := "/home/me/.ccbox"
		cli := harness.CLI{Name: "opencode", DataBinds: map[string]*string{
			".local/share/opencode/auth.json":     new("{}"), // file with seed content
			".local/share/opencode/sessions.json": nil,       // dir despite the extension
			".config/opencode":                    nil,       // dir
		}}

		assert.Equal(t, []docker.Mount{
			docker.NewBind(filepath.Join(userDir, "data", "opencode", ".config-opencode"), path.Join(containerHome, ".config/opencode")),
			docker.NewBind(
				filepath.Join(userDir, "data", "opencode", ".local-share-opencode-auth.json"),
				path.Join(containerHome, ".local/share/opencode/auth.json"),
			),
			docker.NewBind(
				filepath.Join(userDir, "data", "opencode", ".local-share-opencode-sessions.json"),
				path.Join(containerHome, ".local/share/opencode/sessions.json"),
			),
		}, cliDataBinds(userDir, cli))
	})

	t.Run("no binds when the CLI has none", func(t *testing.T) {
		assert.Empty(t, cliDataBinds("/home/me/.ccbox", harness.CLI{Name: "mycli"}))
	})
}

func TestCliTmpBind(t *testing.T) {
	t.Run("binds the scratch dir at cliTmpMount", func(t *testing.T) {
		for _, cliName := range []string{"claude", "codex", "opencode"} {
			cli := harness.MustFor(cliName)
			scratchDir := "/host/.ccbox/tmp/" + cliName
			assert.Equal(t,
				[]docker.Mount{docker.NewBind(scratchDir, cliTmpMount(cli))},
				cliTmpBind(scratchDir, cliTmpMount(cli)), cliName)
		}
	})

	t.Run("no bind when scratchDir is empty (no scratch created)", func(t *testing.T) {
		assert.Empty(t, cliTmpBind("", ""))
	})
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
