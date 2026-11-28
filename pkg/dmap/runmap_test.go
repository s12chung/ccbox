package dmap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/cfg"
	"github.com/s12chung/ccbox/pkg/dmap/share"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/models/cli"
	"github.com/s12chung/ccbox/pkg/models/guiapp"
	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/slug"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/install"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

func TestRunMap_RunOptions(t *testing.T) {
	testutil.Home(t)
	require.NoError(t, share.SafeSeedAgentsMd()) // AgentsMd assumes the shared template doc is seeded

	config, err := cfg.Load(writeConfigYaml(t, "cli: claude@latest\n"+`proxy:
  enabled: true
  allowlist:
    - ccbox-defaults
    - example.com
`), cfg.Flags{})
	require.NoError(t, err)
	rm := NewRunMap(config, runtime.AllCacheVolumes(), false)

	// the pieces RunOptions composes
	hostOptions, hostClean, err := rm.HostOptions(true)
	require.NoError(t, err)
	defer func() { require.NoError(t, hostClean()) }()
	env, err := rm.Env()
	require.NoError(t, err)

	options, clean, err := rm.RunOptions(NewProxyMap(config).Options(func(string) {}))
	require.NoError(t, err)
	require.NotNil(t, clean)
	defer func() { require.NoError(t, clean()) }()

	assert.Equal(t, hostOptions, options.RunHostOptions)
	assert.Equal(t, env, options.Env)
	assert.Equal(t, docker.ImageTag, options.Tag)
	assert.Equal(t, devboxName(config.ProjectDir(), false, true), options.Name)
	assert.Equal(t, docker.HoldersDir(options.Name), options.HoldersDir)
	assert.Equal(t, devboxLogPath(config.ProjectDir(), false, true), options.LogFile)
	assert.False(t, options.ServeVNC)
	assert.Nil(t, options.Foreground, "the foreground view is the command's, not the map's")
	require.NotNil(t, options.Proxy)
	assert.NotNil(t, options.Proxy.BeforeStart)
	assert.NotNil(t, options.Proxy.OnRefresh)
	assert.NotNil(t, options.Proxy.OnStop)
	options.Proxy.BeforeStart = nil
	options.Proxy.OnRefresh = nil
	options.Proxy.OnStop = nil
	assert.Equal(t, docker.ProxyOptions{HostDir: proxyLiveDir(), LogFile: proxyLogPath(config.ProjectDir(), false)}, *options.Proxy)
}

func TestRunMap_RunOptions_VNC(t *testing.T) {
	testutil.Home(t)
	require.NoError(t, share.SafeSeedAgentsMd()) // AgentsMd assumes the shared template doc is seeded

	// the base layer carries the gui_app, as a project's config would
	config, err := cfg.Load(writeConfigYaml(t, "cli: claude@latest\n"+`vnc:
  enabled: true
  gui_app: `+guiapp.App.Name+`@latest
proxy:
  enabled: true
  allowlist:
    - ccbox-defaults
    - ccbox-set-harness
`), cfg.Flags{VNC: true})
	require.NoError(t, err)

	options, clean, err := NewRunMap(config, runtime.AllCacheVolumes(), false).RunOptions(nil)
	require.NoError(t, err)
	require.NotNil(t, clean)
	defer func() { require.NoError(t, clean()) }()

	assert.Equal(t, docker.ImageTag+"-vnc", options.Tag, "the run uses the desktop variant's image: the resolved load, not a flag")
	assert.Equal(t, devboxName(config.ProjectDir(), true, false), options.Name)
	assert.Equal(t, devboxLogPath(config.ProjectDir(), true, false), options.LogFile)
	assert.True(t, options.ServeVNC)
	assert.Nil(t, options.Proxy)
	assert.Contains(t, options.Env, pkginfo.VNCConfigEnvVar, "the VNC env is set even without a vnc config section")
	assert.Equal(t, "/home/ccbox/.zcode/v2/credential-secret", options.Env["ZCODE_CREDENTIAL_SECRET_FILE"],
		"the desktop run's container env carries the GUI app's env")
	expanded := config.AllowlistExpanded()
	for _, d := range guiapp.App.Setup.Proxied.AllowDomains {
		assert.Containsf(t, expanded, d, "the GUI app's download domain is allowlisted via the harness alias")
	}
}

func TestRunMap_RunOptions_VNC_Proxied(t *testing.T) {
	testutil.Home(t)
	require.NoError(t, share.SafeSeedAgentsMd()) // AgentsMd assumes the shared template doc is seeded

	// the --vnc flag on a config without a vnc section, as `ccbox proxy --vnc` arrives
	config, err := cfg.Load(writeConfigYaml(t, "cli: claude@latest\n"), cfg.Flags{VNC: true, ProxyCmd: true})
	require.NoError(t, err)

	options, clean, err := NewRunMap(config, runtime.AllCacheVolumes(), false).RunOptions(NewProxyMap(config).Options(func(string) {}))
	require.NoError(t, err)
	require.NotNil(t, clean)
	defer func() { require.NoError(t, clean()) }()

	assert.Equal(t, docker.ImageTag+"-vnc", options.Tag, "the tag carries the vnc suffix only — the egress wall is runtime-only")
	assert.Equal(t, devboxName(config.ProjectDir(), config.IsVNC(), true), options.Name, "the egress wall makes the proxied vnc combo its own devbox")
	assert.Equal(t, devboxLogPath(config.ProjectDir(), config.IsVNC(), true), options.LogFile)
	assert.True(t, options.ServeVNC)
	assert.NotNil(t, options.Proxy)
}

func TestRunMap_RunOptions_PortBindings(t *testing.T) {
	testutil.Home(t)
	require.NoError(t, share.SafeSeedAgentsMd()) // AgentsMd assumes the shared template doc is seeded

	config, err := cfg.Load(writeConfigYaml(t, "cli: claude@latest\n"+
		"default_variant:\n  port_bindings:\n    tcp: 3000,7200\n"+
		"proxy:\n  enabled: true\n"), cfg.Flags{})
	require.NoError(t, err)

	// the default ask — the config's section applies to its devbox
	options, clean, err := NewRunMap(config, nil, false).RunOptions(NewProxyMap(config).Options(func(string) {}))
	require.NoError(t, err)
	require.NotNil(t, clean)
	defer func() { require.NoError(t, clean()) }()
	assert.Equal(t, []string{"3000", "7200"}, options.PortBindingsTCP)
	runConfig, err := pkginfo.RunConfigFromJSON(options.Env[pkginfo.RunConfigEnvVar])
	require.NoError(t, err)
	assert.Equal(t, []string{"3000", "7200"}, runConfig.PortBindingsTCP, "the egress wall's boot opens each into the workload's netns")
}

func TestRunMap_RunOptions_PortBindings_OtherAsks(t *testing.T) {
	testutil.Home(t)
	require.NoError(t, share.SafeSeedAgentsMd())

	// the --no-proxy ask boots another devbox: the config's section doesn't apply to it
	config, err := cfg.Load(writeConfigYaml(t, "cli: claude@latest\n"+
		"default_variant:\n  port_bindings:\n    tcp: 3000,7200\n"+
		"proxy:\n  enabled: true\n"), cfg.Flags{NoProxy: true})
	require.NoError(t, err)

	options, clean, err := NewRunMap(config, nil, false).RunOptions(nil)
	require.NoError(t, err)
	require.NotNil(t, clean)
	defer func() { require.NoError(t, clean()) }()
	assert.Empty(t, options.PortBindingsTCP)
}

func TestRunMap_RunOptions_PortBindings_FlagsSetDefaultVariant(t *testing.T) {
	testutil.Home(t)
	require.NoError(t, share.SafeSeedAgentsMd())

	// a --vnc ask on a vnc-enabled config lands on the config's own default variant
	config, err := cfg.Load(writeConfigYaml(t, "cli: claude@latest\n"+
		"default_variant:\n  port_bindings:\n    tcp: 3000,7200\n"+
		"vnc:\n  enabled: true\n"+
		"proxy:\n  enabled: true\n"), cfg.Flags{VNC: true})
	require.NoError(t, err)

	options, clean, err := NewRunMap(config, nil, false).RunOptions(NewProxyMap(config).Options(func(string) {}))
	require.NoError(t, err)
	require.NotNil(t, clean)
	defer func() { require.NoError(t, clean()) }()
	assert.True(t, options.ServeVNC)
	assert.Equal(t, []string{"3000", "7200"}, options.PortBindingsTCP)
}

func TestRunMap_RunOptions_PortBindings_FlagOnAnyAsk(t *testing.T) {
	testutil.Home(t)
	require.NoError(t, share.SafeSeedAgentsMd())

	// the --port-bindings-tcp flag on a --vnc ask, as `ccbox run --vnc --port-bindings-tcp 8080` arrives
	config, err := cfg.Load(writeConfigYaml(t, "cli: claude@latest\n"), cfg.Flags{VNC: true, PortBindingsTCP: new("8080")})
	require.NoError(t, err)

	options, clean, err := NewRunMap(config, nil, false).RunOptions(nil)
	require.NoError(t, err)
	require.NotNil(t, clean)
	defer func() { require.NoError(t, clean()) }()
	assert.Equal(t, []string{"8080"}, options.PortBindingsTCP)
}

func TestRunMap_RunOptions_HostGateway(t *testing.T) {
	testutil.Home(t)
	require.NoError(t, share.SafeSeedAgentsMd())

	// the flag's ask over the section, as `ccbox run --host-gateway-tcp 3306,6379` arrives —
	// proxied, where the egress wall's boot forwards each onto the host
	config, err := cfg.Load(writeConfigYaml(t, "cli: claude@latest\n"+
		"host_gateway:\n  tcp: 5432\n"+"proxy:\n  enabled: true\n"), cfg.Flags{HostGatewayTCP: new("3306,6379")})
	require.NoError(t, err)

	options, clean, err := NewRunMap(config, nil, false).RunOptions(NewProxyMap(config).Options(func(string) {}))
	require.NoError(t, err)
	require.NotNil(t, clean)
	defer func() { require.NoError(t, clean()) }()
	assert.Equal(t, []string{"3306", "6379"}, options.HostGatewayTCP)
	runConfig, err := pkginfo.RunConfigFromJSON(options.Env[pkginfo.RunConfigEnvVar])
	require.NoError(t, err)
	assert.Equal(t, []string{"3306", "6379"}, runConfig.HostGatewayTCP,
		"the egress wall's boot forwards each onto the host")
}

func TestRunMap_RunOptions_TagByProject(t *testing.T) {
	testutil.Home(t)
	require.NoError(t, share.SafeSeedAgentsMd()) // AgentsMd assumes the shared template doc is seeded

	projectDir := writeConfigYaml(t, "cli: claude@latest\n"+`proxy:
  allowlist:
    - ccbox-defaults
    - example.com
`)
	config, err := cfg.Load(projectDir, cfg.Flags{})
	require.NoError(t, err)

	options, clean, err := NewRunMap(config, runtime.AllCacheVolumes(), true).RunOptions(nil)
	require.NoError(t, err)
	require.NotNil(t, clean)
	defer func() { require.NoError(t, clean()) }()

	assert.Equal(t, docker.ImageTag+slug.Path(projectDir), options.Tag)
}

func TestRunMap_HostOptions(t *testing.T) {
	home := testutil.Home(t)
	require.NoError(t, share.SafeSeedAgentsMd()) // AgentsMd assumes the shared template doc is seeded

	projectDir := writeConfigYaml(t, "cli: pi@latest\n"+`tmpfs_masks:
  - dist
  - notes.txt
volume_masks:
  - node_modules
read_only_globs:
  - .env
read_only_binds:
  gitconfig: enabled
  ~/fonts: /home/ccbox/fonts
`)

	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, "dist"), osutil.Dir))
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, "node_modules"), osutil.Dir))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "notes.txt"), []byte("real"), osutil.File))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, ".env"), nil, osutil.File))
	// the binds' host dirs must exist to survive the present-filter
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".config", "git"), osutil.Dir))
	require.NoError(t, os.MkdirAll(filepath.Join(home, "fonts"), osutil.Dir))

	config, err := cfg.Load(projectDir, cfg.Flags{})
	require.NoError(t, err)

	hostOptions, clean, err := NewRunMap(config, runtime.AllCacheVolumes(), false).HostOptions(true)
	require.NoError(t, err)
	require.NotNil(t, clean)
	defer func() { require.NoError(t, clean()) }()

	workspace := "/home/ccbox/" + filepath.Base(projectDir)
	s := slug.Path(projectDir)

	// the composite order: the workspace and host dirs, the shared volumes, then the
	// config's masks and per-app binds, then the shared agents doc's scratch bind
	assert.Equal(t, []docker.Mount{
		docker.NewBind(projectDir, workspace),
		docker.NewBind(filepath.Join(home, ".ccbox", "pi"), "/home/ccbox/.pi/agent"),
		docker.NewBind(filepath.Join(home, ".ccbox", "tmp", "persist", s), "/home/ccbox/.ccbox/persist"),
		docker.NewBind(filepath.Join(home, ".ccbox", "skills"), "/home/ccbox/.pi/agent/skills"),
		docker.NewBind(proxyLiveDir(), pkginfo.ProxyMount).ReadOnly(),
		docker.NewVolume("ccbox-clis", install.DefaultRoot).Global(),
		docker.NewVolume("ccbox"+s+"-cache-cache-default", "/home/ccbox/.cache"),
		docker.NewVolume("ccbox"+s+"-cargo-cache-default", "/home/ccbox/.cargo"),
		docker.NewVolume("ccbox"+s+"-go-cache-default", "/home/ccbox/go"),
		docker.NewVolume("ccbox"+s+"-local-cache-default", "/home/ccbox/.local"),
		docker.NewVolume("ccbox"+s+"-npm--global-cache-default", "/home/ccbox/.npm-global"),
		docker.NewVolume("ccbox"+s+"-npm-cache-default", "/home/ccbox/.npm"),
		docker.NewVolume("ccbox"+s+"-tmp-cache-default", "/tmp"),
		docker.NewBind("/dev/null", workspace+"/notes.txt"),
		docker.NewVolume("ccbox"+s+"-node_modules", workspace+"/node_modules"),
		docker.NewBind(filepath.Join(projectDir, ".env"), workspace+"/.env").ReadOnly(),
		docker.NewBind(filepath.Join(home, ".config", "git"), cfg.GitConfigMount).ReadOnly(),
		docker.NewBind(filepath.Join(home, "fonts"), "/home/ccbox/fonts").ReadOnly(),
		docker.NewBind(filepath.Join(home, ".ccbox", "tmp", "agents", "pi", "AGENTS.md"), "/home/ccbox/.ccbox/tmp/agents/pi/AGENTS.md"),
	}, hostOptions.Mounts)

	assert.Equal(t, workspace, hostOptions.WorkspaceMount)
	assert.Equal(t, []string{workspace + "/dist", miseDataMount}, hostOptions.TmpfsPaths, "the file mask is no tmpfs")
}

func TestRunMap_HostOptions_MiseDataTmpfs(t *testing.T) {
	// the mask is always on: an ad-hoc `mise use`'s installs must not persist into
	// the local volume and shadow the system installs the config pins
	testutil.Home(t)
	require.NoError(t, share.SafeSeedAgentsMd())

	config, err := cfg.Load(writeConfigYaml(t, "cli: pi@latest\n"), cfg.Flags{})
	require.NoError(t, err)

	hostOptions, clean, err := NewRunMap(config, runtime.AllCacheVolumes(), false).HostOptions(true)
	require.NoError(t, err)
	defer func() { require.NoError(t, clean()) }()

	assert.Equal(t, []string{miseDataMount}, hostOptions.TmpfsPaths)
}

func TestRunMap_HostOptions_NoProxy(t *testing.T) {
	testutil.Home(t)
	require.NoError(t, share.SafeSeedAgentsMd())

	config, err := cfg.Load(writeConfigYaml(t, "cli: pi@latest\n"), cfg.Flags{})
	require.NoError(t, err)

	hostOptions, clean, err := NewRunMap(config, runtime.AllCacheVolumes(), false).HostOptions(false)
	require.NoError(t, err)
	require.NotNil(t, clean)
	defer func() { require.NoError(t, clean()) }()

	assert.NotContains(t, hostOptions.Mounts,
		docker.NewBind(proxyLiveDir(), pkginfo.ProxyMount).ReadOnly(),
		"a no-proxy run has no proxy, so no allow-file bind")
}

func TestRunMap_HostOptions_VNC(t *testing.T) {
	testutil.Home(t)
	require.NoError(t, share.SafeSeedAgentsMd()) // AgentsMd assumes the shared template doc is seeded

	config, err := cfg.Load(writeConfigYaml(t, "cli: claude@latest\nvnc:\n  enabled: true\n"), cfg.Flags{})
	require.NoError(t, err)

	hostOptions, clean, err := NewRunMap(config, runtime.AllCacheVolumes(), false).HostOptions(true)
	require.NoError(t, err)
	require.NotNil(t, clean)
	defer func() { require.NoError(t, clean()) }()

	assert.Contains(t, hostOptions.Mounts, docker.NewVolume("ccbox-apps", install.AppsRoot).Global(),
		"the GUI app's install volume is mounted only on a VNC run")
}

func TestRunMap_Env(t *testing.T) {
	t.Setenv("GH_TOKEN", "tok")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")
	t.Setenv("DISABLE_AUTOUPDATER", "0")
	rm := testRunMap(t, "cli: claude@latest\n"+`forward_env:
  - TERM
  - COLORTERM
  - GH_TOKEN
  - DISABLE_AUTOUPDATER
vnc:
  enabled: true
  gui_app: `+guiapp.App.Name+`@latest
`)
	env, err := rm.Env()
	require.NoError(t, err)

	pkgInfo, err := cli.MustFor("claude").PkgInfoJSON()
	require.NoError(t, err)
	vncInfo, err := rm.cfg.VNC.InfoJSON()
	require.NoError(t, err)
	//nolint:gosec // the map carries env var names, not credentials
	assert.Equal(t, map[string]string{
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1", // from CLI.yaml
		"DISABLE_AUTOUPDATER":                      "0", // forwarded, overrides CLI.yaml's from above
		"TERM":                                     "xterm-256color",
		"COLORTERM":                                "truecolor",
		pkginfo.EnvVar:                             pkgInfo,
		pkginfo.RunConfigEnvVar:                    `{"cli_version":"latest","gui_app_version":"latest"}`,
		pkginfo.VNCConfigEnvVar:                    vncInfo,
		"GH_TOKEN":                                 "tok",
		"XDG_DATA_DIRS":                            "/home/ccbox/.ccbox/xdg/share:/usr/local/share:/usr/share", // from the GUI app
		"ZCODE_CREDENTIAL_SECRET_FILE":             "/home/ccbox/.zcode/v2/credential-secret",                  // from the GUI app
	}, env)
}

func TestRunMap_Env_PinnedVersion(t *testing.T) {
	rm := testRunMap(t, "cli: claude@0.9.2\n")

	env, err := rm.Env()
	require.NoError(t, err)
	runConfig, err := pkginfo.RunConfigFromJSON(env[pkginfo.RunConfigEnvVar])
	require.NoError(t, err)
	assert.Equal(t, "0.9.2", runConfig.CLIVersion, "the config carries the ask")
	info, err := pkginfo.FromJSON(env[pkginfo.EnvVar])
	require.NoError(t, err)
	assert.Equal(t, pkginfo.PkgInfo{Name: "claude", Npm: &pkginfo.Npm{Package: "@anthropic-ai/claude-code"}}, info,
		"the install source carries no version")
}

func TestRunMap_Env_PinnedGUIAppVersion(t *testing.T) {
	rm := testRunMap(t, "cli: claude@latest\nvnc:\n  enabled: true\n  gui_app: "+guiapp.App.Name+"@1.2.3\n")

	env, err := rm.Env()
	require.NoError(t, err)
	runConfig, err := pkginfo.RunConfigFromJSON(env[pkginfo.RunConfigEnvVar])
	require.NoError(t, err)
	assert.Equal(t, new("1.2.3"), runConfig.GUIAppVersion, "the config carries the GUI app's ask")
	info, err := pkginfo.VNCInfoFromJSON(env[pkginfo.VNCConfigEnvVar])
	require.NoError(t, err)
	assert.Equal(t, guiapp.App.GUIPkgInfo, *info.GUIApp, "the install source carries no version")
}

func TestRunMap_Env_SkipsEmpty(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("TERM", "")
	t.Setenv("COLORTERM", "")
	rm := testRunMap(t, "cli: claude@latest\n"+"forward_env:\n  - TERM\n  - COLORTERM\n  - GH_TOKEN\n")

	env, err := rm.Env()
	require.NoError(t, err)
	assert.NotContains(t, env, "TERM")
	assert.NotContains(t, env, "COLORTERM")
	assert.NotContains(t, env, "GH_TOKEN", "an unset var forwards nothing too")
}

func TestRunMap_Env_SkipsUnsetVNC(t *testing.T) {
	rm := testRunMap(t, "cli: claude@latest\n")

	env, err := rm.Env()
	require.NoError(t, err)
	assert.NotContains(t, env, pkginfo.VNCConfigEnvVar)
}

func TestRunMap_Env_EmptyVNC(t *testing.T) {
	rm := testRunMap(t, "cli: claude@latest\n"+"vnc:\n  enabled: true\n")

	env, err := rm.Env()
	require.NoError(t, err)
	assert.Equal(t, "{}", env[pkginfo.VNCConfigEnvVar], "an empty vnc section serves the desktop with no GUI app")
	runConfig, err := pkginfo.RunConfigFromJSON(env[pkginfo.RunConfigEnvVar])
	require.NoError(t, err)
	assert.Nil(t, runConfig.GUIAppVersion, "no GUI app, no version ask")
}

func TestRunMap_Env_PortBindings(t *testing.T) {
	rm := testRunMap(t, "cli: claude@latest\n"+"default_variant:\n  port_bindings:\n    tcp: 3000,7200\n")

	env, err := rm.Env()
	require.NoError(t, err)
	runConfig, err := pkginfo.RunConfigFromJSON(env[pkginfo.RunConfigEnvVar])
	require.NoError(t, err)
	assert.Equal(t, []string{"3000", "7200"}, runConfig.PortBindingsTCP)

	rm = testRunMap(t, "cli: claude@latest\n")
	env, err = rm.Env()
	require.NoError(t, err)
	assert.JSONEq(t, `{"cli_version":"latest"}`, env[pkginfo.RunConfigEnvVar], "unasked ports add no key")
}

func TestRunMap_Env_HostGateway(t *testing.T) {
	rm := testRunMap(t, "cli: claude@latest\n"+"host_gateway:\n  tcp: 5432,6379\n")

	env, err := rm.Env()
	require.NoError(t, err)
	runConfig, err := pkginfo.RunConfigFromJSON(env[pkginfo.RunConfigEnvVar])
	require.NoError(t, err)
	assert.Equal(t, []string{"5432", "6379"}, runConfig.HostGatewayTCP, "the egress wall's boot forwards each onto the host")

	rm = testRunMap(t, "cli: claude@latest\n")
	env, err = rm.Env()
	require.NoError(t, err)
	assert.JSONEq(t, `{"cli_version":"latest"}`, env[pkginfo.RunConfigEnvVar], "unasked ports add no key")
}
