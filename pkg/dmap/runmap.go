package dmap

import (
	"os"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/kit/tinyproxy"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/klean"
	"github.com/s12chung/ccbox/pkg/util/mergeempty"
)

// envGHToken passes the host's GitHub token through to the container for gh.
const envGHToken = "GH_TOKEN"

// envTerminalVars forwards the host terminal's vars
var envTerminalVars = []string{"TERM", "COLORTERM"}

// RunMap maps one run's project config to the docker pkg options
type RunMap struct {
	userDir string // the run's mounts sit under this ccbox per-user host dir
	cfg     *projectcfg.Config
}

// NewRunMap returns a new RunMap
func NewRunMap(userDir string, cfg *projectcfg.Config) *RunMap {
	return &RunMap{userDir: userDir, cfg: cfg}
}

// RunFlags are the run command's CLI inputs that shape the run's docker options
type RunFlags struct {
	Tag   string
	Args  []string
	Modes RunModes
}

// RunModes are the run command's boolean mode flags
type RunModes struct {
	Shell    bool // drop into the image's default shell instead of launching the CLI
	Continue bool // -c: continue the last session
	Resume   bool // -r: resume a session
	NoProxy  bool // skip the egress wall: direct network access
	VNC      bool // serve the config's desktop over VNC
}

// RunOptions renders the run's full docker.RunOptions
func (rm *RunMap) RunOptions(runFlags RunFlags) (docker.RunOptions, func() error, error) {
	noProxy := rm.cfg.NoProxyFor(runFlags.Modes.VNC, runFlags.Modes.NoProxy)
	var joiner klean.Joiner
	hostOptions, clean, err := rm.HostOptions(runFlags.Modes.VNC, noProxy)
	joiner.Push("clean run files", clean)
	if err != nil {
		return docker.RunOptions{}, joiner.Run, err
	}
	env, err := rm.Env(runFlags.Modes.VNC)
	if err != nil {
		return docker.RunOptions{}, joiner.Run, err
	}

	var proxyOptions *docker.ProxyOptions
	if !noProxy {
		logFile, err := os.OpenFile(proxyLogPath(rm.userDir), os.O_APPEND|os.O_CREATE|os.O_WRONLY, ioutil.File)
		if err != nil {
			return docker.RunOptions{}, joiner.Run, err
		}
		joiner.Push("close proxy log file", logFile.Close)

		proxyOptions = NewProxyMap(rm.cfg).Options(&tinyproxy.Log{Writer: logFile})
	}
	return docker.RunOptions{
		RunHostOptions: hostOptions,
		Tag:            variantTag(runFlags.Tag, runFlags.Modes.VNC),
		Env:            env,
		Cmd:            rm.cmd(runFlags),
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
func (rm *RunMap) vncCfg() *projectcfg.VNC {
	if rm.cfg.VNC == nil {
		return &projectcfg.VNC{}
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
	env := mergeempty.Map(cli.Env, hostTerminalEnv())
	env = mergeempty.Map(mergeempty.Map(env, rm.cfg.Env), map[string]string{
		pkginfo.EnvVar: pkgInfo,
		envGHToken:     os.Getenv(envGHToken),
	})
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

// hostTerminalEnv reads envTerminalVars, skipping unset ones so docker's TERM default stands —
// an empty TERM renders worse than that default.
func hostTerminalEnv() map[string]string {
	env := make(map[string]string, len(envTerminalVars))
	for _, k := range envTerminalVars {
		if v := os.Getenv(k); v != "" {
			env[k] = v
		}
	}
	return env
}

// cmd maps the run flags to the CLI's session launch argv.
func (rm *RunMap) cmd(flags RunFlags) []string {
	return rm.cfg.CLI().SessionCmd(flags.Modes.Shell, flags.Modes.Continue, flags.Modes.Resume, flags.Args)
}
