package dmap

import (
	"os"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/pkg/docker"
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
}

// RunOptions renders the run's full docker.RunOptions
func (rm *RunMap) RunOptions(flags RunFlags) (docker.RunOptions, func() error, error) {
	hostOptions, clean, err := rm.HostOptions()
	if err != nil {
		return docker.RunOptions{}, clean, err
	}
	env, err := rm.Env()
	if err != nil {
		return docker.RunOptions{}, clean, err
	}

	return docker.RunOptions{
		RunHostOptions: hostOptions,
		Tag:            flags.Tag,
		Env:            env,
		Cmd:            rm.Cmd(flags),
		Proxy:          NewProxyMap(rm.cfg).Options(),
		ProxyLogPath:   ProxyLogPath(rm.userDir),
		NoProxy:        flags.Modes.NoProxy,
	}, clean, nil
}

// HostOptions renders the run's host options for docker.Run.
func (rm *RunMap) HostOptions() (docker.RunHostOptions, func() error, error) {
	projectDir := rm.cfg.ProjectDir()
	binds, clean, err := rm.binds()
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

// Env renders the container's env in one map: the CLI's fixed env and the host's terminal
// passthrough under the config's overrides, plus the CLI's pkginfo and the host's GitHub token.
func (rm *RunMap) Env() (map[string]string, error) {
	cli := rm.cfg.CLI()
	pkgInfo, err := cli.PkgInfoJSON()
	if err != nil {
		return nil, err
	}
	env := mergeempty.Map(cli.Env, hostTerminalEnv())
	return mergeempty.Map(mergeempty.Map(env, rm.cfg.Env), map[string]string{
		pkginfo.EnvVar: pkgInfo,
		envGHToken:     os.Getenv(envGHToken),
	}), nil
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
