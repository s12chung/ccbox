package dmap

import (
	"os"

	"github.com/s12chung/ccbox/pkg/cfg"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/util/klean"
	"github.com/s12chung/ccbox/pkg/util/mergeempty"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

// RunMap maps one run's project config to the docker pkg options
type RunMap struct {
	userDir      string // the run's mounts sit under this ccbox per-user host dir
	cfg          *cfg.Config
	tagByProject bool
}

// NewRunMap returns a new RunMap
func NewRunMap(userDir string, cfg *cfg.Config, tagByProject bool) *RunMap {
	return &RunMap{userDir: userDir, cfg: cfg, tagByProject: tagByProject}
}

// RunMode is the run's selected mode, with Args carrying its args
type RunMode struct {
	Shell    bool     // the run subcommand: Args exec in the container
	Continue bool     // -c: continue the last session
	Resume   bool     // -r: resume a session
	NoProxy  bool     // skip the egress wall: direct network access
	Args     []string // the exec argv (run), or the resumed session's name (-r)
}

// RunOptions renders the run's full docker.RunOptions
func (rm *RunMap) RunOptions(tag string, mode RunMode) (docker.RunOptions, func() error, error) {
	serveVNC := rm.cfg.ServeVNC()
	noProxy := rm.cfg.NoProxyFor(serveVNC, mode.NoProxy)
	var joiner klean.Joiner
	hostOptions, clean, err := rm.HostOptions(serveVNC, noProxy)
	joiner.Push("clean run files", clean)
	if err != nil {
		return docker.RunOptions{}, joiner.Run, err
	}
	env, err := rm.Env(serveVNC)
	if err != nil {
		return docker.RunOptions{}, joiner.Run, err
	}

	var proxyOptions *docker.ProxyOptions
	if !noProxy {
		proxyOptions = NewProxyMap(rm.cfg).Options(nil)
	}
	return docker.RunOptions{
		RunHostOptions: hostOptions,
		Tag:            tag + tagSuffix(rm.cfg.ProjectDir(), rm.tagByProject, serveVNC),
		Env:            env,
		Cmd:            rm.cmd(mode),
		Proxy:          proxyOptions,
	}, joiner.Run, nil
}

// HostOptions renders the run's host options for docker.Run; vnc mounts the
// GUI app's install volume, which seeds from the desktop variant's image.
func (rm *RunMap) HostOptions(serveVNC, noProxy bool) (docker.RunHostOptions, func() error, error) {
	projectDir := rm.cfg.ProjectDir()
	binds, clean, err := rm.binds(serveVNC, noProxy)
	if err != nil {
		return docker.RunHostOptions{}, clean, err
	}

	return docker.RunHostOptions{
		ProjectDir:     projectDir,
		WorkspaceMount: workspaceMount(projectDir),
		Mounts:         binds,
		TmpfsPaths:     tmpfsMasks(projectDir, rm.cfg.TmpfsMasksPresent()),
	}, clean, nil
}

// vncCfg returns the config's vnc section — an empty one when the config carries none
func (rm *RunMap) vncCfg() *cfg.VNC {
	if rm.cfg.VNC == nil {
		return &cfg.VNC{}
	}
	return rm.cfg.VNC
}

// Env renders the container's env in one map.
func (rm *RunMap) Env(serveVNC bool) (map[string]string, error) {
	cli := rm.cfg.CLI()
	pkgInfo, err := cli.PkgInfoJSON()
	if err != nil {
		return nil, err
	}
	env := mergeempty.Map(cli.Env, forwardEnv(rm.cfg.ForwardEnv))
	env = mergeempty.Map(env, map[string]string{pkginfo.EnvVar: pkgInfo})
	if serveVNC {
		body, err := rm.vncCfg().InfoJSON()
		if err != nil {
			return nil, err
		}
		// pkginfo.VNCConfigEnvVar's presence — even `{}` from a config without a vnc section — is the
		// container's cue to serve the desktop; a headless run sends none, its config's
		// vnc section left unused, unwarned.
		env[pkginfo.VNCConfigEnvVar] = body
	}
	return env, nil
}

// forwardEnv reads the config's vars off the host, skipping empty ones
func forwardEnv(names []string) map[string]string {
	env := make(map[string]string, len(names))
	for _, name := range names {
		if v := os.Getenv(name); v != "" {
			env[name] = v
		}
	}
	return env
}

// cmd maps the run mode to the container's Cmd: the run command's exec argv —
// nil for the image's default shell — or the CLI's session launch argv.
func (rm *RunMap) cmd(mode RunMode) []string {
	if mode.Shell {
		return mode.Args
	}
	return rm.cfg.CLI().SessionCmd(mode.Continue, mode.Resume, mode.Args)
}
