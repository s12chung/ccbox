package dmap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/ccboxtools/pkg/install"
	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/slug"
)

func TestNewRunMap(t *testing.T) {
	t.Run("ResolvesConfigCLI", func(t *testing.T) {
		rm := testRunMap(t, projectcfg.Config{CLI: new("claude")})
		assert.Equal(t, harness.MustFor("claude"), rm.cli)
	})
	t.Run("UnknownCLIPanics", func(t *testing.T) {
		assert.PanicsWithValue(t, `harness: unknown cli "emacs"`, func() {
			NewRunMap(t.TempDir(), &projectcfg.Config{CLI: new("emacs")})
		})
	})
}

func TestRunMap_HostOptions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	require.NoError(t, harness.SafeSeedAgentsMd()) // AgentsMdShare assumes the shared doc is seeded

	cwd := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(cwd, "dist"), ioutil.Dir))
	require.NoError(t, os.MkdirAll(filepath.Join(cwd, "node_modules"), ioutil.Dir))
	require.NoError(t, os.WriteFile(filepath.Join(cwd, ".env"), nil, ioutil.File))

	userDir := t.TempDir()
	cfg, err := projectcfg.Load(cwd, projectcfg.Config{
		CLI:           new("codex"),
		HostGitConfig: new(false),
		TmpfsMasks:    []string{"dist"},
		VolumeMasks:   []string{"node_modules"},
		ReadOnlyGlobs: []string{".env"},
	})
	require.NoError(t, err)

	hostOptions, clean, err := NewRunMap(userDir, cfg).HostOptions()
	require.NoError(t, err)
	require.NotNil(t, clean)
	defer func() { require.NoError(t, clean()) }()

	workspace := "/home/ccbox/" + filepath.Base(cwd)
	s := slug.Path(cwd)

	// the composite order: the workspace and host dirs, the shared volumes, then the
	// config's masks and per-CLI binds, then the shared agents doc's scratch bind
	assert.Equal(t, []docker.Mount{
		docker.NewBind(cwd, workspace),
		docker.NewBind(filepath.Join(userDir, "codex"), "/home/ccbox/.codex"),
		docker.NewBind(filepath.Join(userDir, "projects", s), "/home/ccbox/.ccbox/project"),
		docker.NewVolume("ccbox-clis", install.DefaultRoot).Global(),
		docker.NewVolume("ccbox"+s+"-cache", "/home/ccbox/.cache"),
		docker.NewVolume("ccbox"+s+"-gem", "/home/ccbox/.gem"),
		docker.NewVolume("ccbox"+s+"-go", "/home/ccbox/go"),
		docker.NewVolume("ccbox"+s+"-local", "/home/ccbox/.local"),
		docker.NewVolume("ccbox"+s+"-npm", "/home/ccbox/.npm"),
		docker.NewVolume("ccbox"+s+"-npm-global", "/home/ccbox/.npm-global"),
		docker.NewVolume("ccbox"+s+"-tmp", "/tmp"),
		docker.NewVolume("ccbox"+s+"-node_modules", workspace+"/node_modules").Owned(),
		docker.NewBind(filepath.Join(cwd, ".env"), workspace+"/.env").ReadOnly(),
		docker.NewBind(filepath.Join(home, ".ccbox", "tmp", "codex", "AGENTS.md"), "/home/ccbox/.codex/AGENTS.md"),
	}, hostOptions.Mounts)

	assert.Equal(t, workspace, hostOptions.WorkspaceMountPath)
	assert.Equal(t, []string{workspace + "/dist"}, hostOptions.TmpfsPaths)
}

func TestRunMap_HostOptions_MaskErrorStillSettles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	require.NoError(t, harness.SafeSeedAgentsMd())

	cwd := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(filepath.Dir(cwd), "escape"), ioutil.Dir))

	cfg, err := projectcfg.Load(cwd, projectcfg.Config{
		CLI:           new("codex"),
		HostGitConfig: new(false),
	})
	require.NoError(t, err)
	cfg.TmpfsMasks = []string{"../escape"} // set post-Load to create an error

	// the mask error comes after binds() began the scratch: clean still settles it
	_, clean, err := NewRunMap(t.TempDir(), cfg).HostOptions()
	require.Error(t, err)
	require.NoError(t, clean())

	cli := harness.MustFor("codex")
	_, statErr := os.Stat(filepath.Join(home, ".ccbox", "tmp", cli.Name, cli.SeedAgentsFilename))
	assert.True(t, os.IsNotExist(statErr))
}

func TestRunMap_Env(t *testing.T) {
	t.Setenv("GH_TOKEN", "tok")
	rm := testRunMap(t, projectcfg.Config{
		CLI: new("claude"),
		Env: map[string]string{"DISABLE_AUTOUPDATER": "0"},
	})
	env, err := rm.Env()
	require.NoError(t, err)

	pkgInfo, err := harness.MustFor("claude").PkgInfoJSON()
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1", // from CLI.yaml
		"DISABLE_AUTOUPDATER":                      "0", // overrides CLI.yaml from above
		pkginfo.EnvVar:                             pkgInfo,
		"GH_TOKEN":                                 "tok",
	}, env)
}

func TestRunMap_Cmd(t *testing.T) {
	rm := testRunMap(t, projectcfg.Config{CLI: new("claude")})

	assert.Equal(t, []string{"claude"}, rm.Cmd(false, false, false, nil))
	assert.Nil(t, rm.Cmd(true, false, false, nil), "shell runs the image default instead")
	assert.Equal(t, []string{"claude", "-c"}, rm.Cmd(false, true, false, nil))
	assert.Equal(t, []string{"claude", "--resume", "sess"}, rm.Cmd(false, false, true, []string{"sess"}))
}
