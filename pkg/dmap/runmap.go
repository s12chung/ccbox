package dmap

import (
	"os"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/guiapp"
	"github.com/s12chung/ccbox/pkg/projectcfg"
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
	VNC      bool // serve the config's desktop over VNC; implies NoProxy
}

// RunOptions renders the run's full docker.RunOptions
func (rm *RunMap) RunOptions(flags RunFlags) (docker.RunOptions, func() error, error) {
	hostOptions, clean, err := rm.HostOptions(flags.Modes.VNC)
	if err != nil {
		return docker.RunOptions{}, clean, err
	}
	env, err := rm.Env(flags.Modes.VNC)
	if err != nil {
		return docker.RunOptions{}, clean, err
	}

	return docker.RunOptions{
		RunHostOptions: hostOptions,
		Tag:            variantTag(flags.Tag, flags.Modes.VNC),
		Env:            env,
		Cmd:            rm.Cmd(flags),
		Proxy:          NewProxyMap(rm.cfg).Options(rm.guiAppAllow(flags.Modes.VNC)),
		ProxyLogPath:   ProxyLogPath(rm.userDir),
		NoProxy:        flags.Modes.NoProxy || flags.Modes.VNC,
	}, clean, nil
}

// HostOptions renders the run's host options for docker.Run; vnc mounts the
// GUI app's install volume, which seeds from the desktop variant's image.
func (rm *RunMap) HostOptions(vnc bool) (docker.RunHostOptions, func() error, error) {
	projectDir := rm.cfg.ProjectDir()
	binds, clean, err := rm.binds(vnc)
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

// guiAppAllow returns the GUI app's download domains for the wall — only a
// desktop run downloads the app, so only its gui_app carries domains
func (rm *RunMap) guiAppAllow(vnc bool) []string {
	if !vnc {
		return nil
	}
	return guiapp.AllowDomains(rm.vncCfg().GUIAppName)
}

// Env renders the container's env in one map.
func (rm *RunMap) Env(vnc bool) (map[string]string, error) {
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
	if vnc {
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

// Cmd maps the run flags to the CLI's session launch argv.
func (rm *RunMap) Cmd(flags RunFlags) []string {
	return rm.cfg.CLI().SessionCmd(flags.Modes.Shell, flags.Modes.Continue, flags.Modes.Resume, flags.Args)
}
