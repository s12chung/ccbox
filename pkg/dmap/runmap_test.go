package dmap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/cli"
	"github.com/s12chung/ccbox/pkg/dmap/share"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/guiapp"
	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/slug"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/install"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

func TestRunMap_RunOptions(t *testing.T) {
	testutil.Home(t)
	require.NoError(t, share.SafeSeedAgentsMd()) // AgentsMd assumes the ccbox-admin doc is seeded

	cfg, err := projectcfg.Load(t.TempDir(), projectcfg.Config{
		CLIName:   new("claude"),
		Allowlist: []string{projectcfg.DefaultsAlias, "example.com"},
	}, false)
	require.NoError(t, err)
	userDir := t.TempDir()
	rm := NewRunMap(userDir, cfg)

	// the pieces RunOptions composes
	hostOptions, hostClean, err := rm.HostOptions(false, false)
	require.NoError(t, err)
	defer func() { require.NoError(t, hostClean()) }()
	env, err := rm.Env(false)
	require.NoError(t, err)

	options, clean, err := rm.RunOptions("dev:tag", RunMode{})
	require.NoError(t, err)
	require.NotNil(t, clean)
	defer func() { require.NoError(t, clean()) }()

	assert.Equal(t, hostOptions, options.RunHostOptions)
	assert.Equal(t, env, options.Env)
	assert.Equal(t, "dev:tag", options.Tag)
	assert.Equal(t, []string{"claude"}, options.Cmd)
	require.NotNil(t, options.Proxy)
	assert.NotNil(t, options.Proxy.BeforeStart)
	assert.NotNil(t, options.Proxy.OnRefresh)
	assert.NotNil(t, options.Proxy.OnStop)
	assert.Nil(t, options.Proxy.Log, "runs ride the session's log file, not a foreground stream")
	options.Proxy.BeforeStart = nil
	options.Proxy.OnRefresh = nil
	options.Proxy.OnStop = nil
	assert.Equal(t, docker.ProxyOptions{HostDir: proxyLiveDir(), HoldersDir: proxyHoldersDir(), LogFile: proxyLogPath()}, *options.Proxy)
}

func TestRunMap_RunOptions_VNC(t *testing.T) {
	testutil.Home(t)
	require.NoError(t, share.SafeSeedAgentsMd()) // AgentsMd assumes the ccbox-admin doc is seeded

	// the base layer carries the gui_app, as a project's config would
	cfg, err := projectcfg.Load(t.TempDir(), projectcfg.Config{
		CLIName:   new("claude"),
		VNC:       &projectcfg.VNC{GUIAppName: guiapp.App.Name},
		Allowlist: []string{projectcfg.DefaultsAlias, projectcfg.SetHarnessAlias},
	}, true)
	require.NoError(t, err)

	options, clean, err := NewRunMap(t.TempDir(), cfg).RunOptions("dev:tag", RunMode{})
	require.NoError(t, err)
	require.NotNil(t, clean)
	defer func() { require.NoError(t, clean()) }()

	assert.Equal(t, "dev:tag-vnc", options.Tag, "the run uses the desktop variant's image: the resolved load, not a flag")
	assert.Nil(t, options.Proxy, "a vnc run always skips the egress wall")
	assert.Contains(t, options.Env, pkginfo.VNCConfigEnvVar, "the VNC env rides even without a vnc config section")
	expanded := cfg.AllowlistExpanded()
	for _, d := range guiapp.App.AllowDomains {
		assert.Containsf(t, expanded, d, "the GUI app's download domain rides the proxy via the harness alias")
	}
}

func TestRunMap_HostOptions(t *testing.T) {
	home := testutil.Home(t)
	require.NoError(t, share.SafeSeedAgentsMd()) // AgentsMd assumes the ccbox-admin doc is seeded

	projectDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, "dist"), ioutil.Dir))
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, "node_modules"), ioutil.Dir))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, ".env"), nil, ioutil.File))
	// the binds' host dirs must exist to survive the present-filter
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".config", "git"), ioutil.Dir))
	require.NoError(t, os.MkdirAll(filepath.Join(home, "fonts"), ioutil.Dir))

	cfg, err := projectcfg.Load(projectDir, projectcfg.Config{
		CLIName:       new("codex"),
		TmpfsMasks:    []string{"dist"},
		VolumeMasks:   []string{"node_modules"},
		ReadOnlyGlobs: []string{".env"},
		ReadOnlyBinds: map[string]string{projectcfg.GitConfigKey: firmrule.EnabledValue, "~/fonts": "/home/ccbox/fonts"},
	}, false)
	require.NoError(t, err)

	hostOptions, clean, err := NewRunMap(t.TempDir(), cfg).HostOptions(false, false)
	require.NoError(t, err)
	require.NotNil(t, clean)
	defer func() { require.NoError(t, clean()) }()

	workspace := "/home/ccbox/" + filepath.Base(projectDir)
	s := slug.Path(projectDir)

	// the composite order: the workspace and host dirs, the shared volumes, then the
	// config's masks and per-CLI binds, then the shared agents doc's scratch bind
	assert.Equal(t, []docker.Mount{
		docker.NewBind(projectDir, workspace),
		docker.NewBind(filepath.Join(home, ".ccbox", "codex"), "/home/ccbox/.codex"),
		docker.NewBind(filepath.Join(home, ".ccbox", "tmp", "persist", s), "/home/ccbox/.ccbox/persist"),
		docker.NewBind(proxyLiveDir(), pkginfo.ProxyMount).ReadOnly(),
		docker.NewVolume("ccbox-clis", install.DefaultRoot).Global(),
		docker.NewVolume("ccbox"+s+"-cache-cache-default", "/home/ccbox/.cache"),
		docker.NewVolume("ccbox"+s+"-gem-cache-default", "/home/ccbox/.gem"),
		docker.NewVolume("ccbox"+s+"-go-cache-default", "/home/ccbox/go"),
		docker.NewVolume("ccbox"+s+"-local-cache-default", "/home/ccbox/.local"),
		docker.NewVolume("ccbox"+s+"-npm--global-cache-default", "/home/ccbox/.npm-global"),
		docker.NewVolume("ccbox"+s+"-npm-cache-default", "/home/ccbox/.npm"),
		docker.NewVolume("ccbox"+s+"-tmp-cache-default", "/tmp"),
		docker.NewVolume("ccbox"+s+"-node_modules", workspace+"/node_modules"),
		docker.NewBind(filepath.Join(projectDir, ".env"), workspace+"/.env").ReadOnly(),
		docker.NewBind(filepath.Join(home, ".config", "git"), projectcfg.GitConfigMount).ReadOnly(),
		docker.NewBind(filepath.Join(home, "fonts"), "/home/ccbox/fonts").ReadOnly(),
		docker.NewBind(filepath.Join(home, ".ccbox", "tmp", "agents", "codex", "AGENTS.md"), "/home/ccbox/.ccbox/tmp/agents/codex/AGENTS.md"),
	}, hostOptions.Mounts)

	assert.Equal(t, workspace, hostOptions.WorkspaceMount)
	assert.Equal(t, []string{workspace + "/dist"}, hostOptions.TmpfsPaths)
}

func TestRunMap_HostOptions_NoProxy(t *testing.T) {
	testutil.Home(t)
	require.NoError(t, share.SafeSeedAgentsMd())

	cfg, err := projectcfg.Load(t.TempDir(), projectcfg.Config{CLIName: new("codex")}, false)
	require.NoError(t, err)

	hostOptions, clean, err := NewRunMap(t.TempDir(), cfg).HostOptions(false, true)
	require.NoError(t, err)
	require.NotNil(t, clean)
	defer func() { require.NoError(t, clean()) }()

	assert.NotContains(t, hostOptions.Mounts,
		docker.NewBind(proxyLiveDir(), pkginfo.ProxyMount).ReadOnly(),
		"a no-proxy run has no proxy, so no allow-file bind")
}

func TestRunMap_HostOptions_VNC(t *testing.T) {
	testutil.Home(t)
	require.NoError(t, share.SafeSeedAgentsMd()) // AgentsMd assumes the ccbox-admin doc is seeded

	cfg, err := projectcfg.Load(t.TempDir(), projectcfg.Config{CLIName: new("claude")}, false)
	require.NoError(t, err)

	hostOptions, clean, err := NewRunMap(t.TempDir(), cfg).HostOptions(true, false)
	require.NoError(t, err)
	require.NotNil(t, clean)
	defer func() { require.NoError(t, clean()) }()

	assert.Contains(t, hostOptions.Mounts, docker.NewVolume("ccbox-apps", install.AppsRoot).Global(),
		"the GUI app's install volume rides only a VNC run")
}

func TestRunMap_Env(t *testing.T) {
	t.Setenv("GH_TOKEN", "tok")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")
	t.Setenv("DISABLE_AUTOUPDATER", "0")
	vnc := &projectcfg.VNC{GUIAppName: guiapp.App.Name}
	rm := testRunMap(t, projectcfg.Config{
		CLIName:    new("claude"),
		ForwardEnv: []string{"TERM", "COLORTERM", "GH_TOKEN", "DISABLE_AUTOUPDATER"},
		VNC:        vnc,
	})
	env, err := rm.Env(true)
	require.NoError(t, err)

	pkgInfo, err := cli.MustFor("claude").PkgInfoJSON()
	require.NoError(t, err)
	vncInfo, err := vnc.InfoJSON()
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1", // from CLI.yaml
		"DISABLE_AUTOUPDATER":                      "0", // forwarded, overrides CLI.yaml's from above
		"TERM":                                     "xterm-256color",
		"COLORTERM":                                "truecolor",
		pkginfo.EnvVar:                             pkgInfo,
		pkginfo.VNCConfigEnvVar:                    vncInfo,
		"GH_TOKEN":                                 "tok",
	}, env)
}

func TestRunMap_Env_SkipsEmpty(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("TERM", "")
	t.Setenv("COLORTERM", "")
	rm := testRunMap(t, projectcfg.Config{
		CLIName:    new("claude"),
		ForwardEnv: []string{"TERM", "COLORTERM", "GH_TOKEN"},
	})

	env, err := rm.Env(false)
	require.NoError(t, err)
	assert.NotContains(t, env, "TERM")
	assert.NotContains(t, env, "COLORTERM")
	assert.NotContains(t, env, "GH_TOKEN", "an unset var forwards nothing too")
}

func TestRunMap_Env_SkipsUnsetVNC(t *testing.T) {
	rm := testRunMap(t, projectcfg.Config{CLIName: new("claude")})

	env, err := rm.Env(false)
	require.NoError(t, err)
	assert.NotContains(t, env, pkginfo.VNCConfigEnvVar)
}

func TestRunMap_Env_EmptyVNC(t *testing.T) {
	rm := testRunMap(t, projectcfg.Config{CLIName: new("claude")})

	env, err := rm.Env(true)
	require.NoError(t, err)
	assert.Equal(t, "{}", env[pkginfo.VNCConfigEnvVar], "an empty vnc section serves the desktop with no GUI app")
}

func TestRunMap_Cmd(t *testing.T) {
	rm := testRunMap(t, projectcfg.Config{CLIName: new("claude")})

	assert.Equal(t, []string{"claude"}, rm.cmd(RunMode{}))
	assert.Nil(t, rm.cmd(RunMode{Shell: true}), "bare run drops into the image's default shell")
	assert.Equal(t, []string{"make", "test"},
		rm.cmd(RunMode{Shell: true, Args: []string{"make", "test"}}), "run execs the args")
	assert.Equal(t, []string{"claude", "-c"}, rm.cmd(RunMode{Continue: true}))
	assert.Equal(t, []string{"claude", "--resume", "sess"},
		rm.cmd(RunMode{Resume: true, Args: []string{"sess"}}))
}
