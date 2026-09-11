package dmap

import (
	"os"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/util/cleanup"
	"github.com/s12chung/ccbox/pkg/util/mergeempty"
)

// envGHToken passes the host's GitHub token through to the container for gh.
const envGHToken = "GH_TOKEN"

// RunMap maps one run's project config to the docker pkg options
type RunMap struct {
	userDir string // the run's mounts sit under this ccbox per-user host dir
	cfg     *projectcfg.Config
	cli     harness.CLI
}

// NewRunMap returns a new RunMap
func NewRunMap(userDir string, cfg *projectcfg.Config) *RunMap {
	return &RunMap{userDir: userDir, cfg: cfg, cli: harness.MustFor(*cfg.CLIName)}
}

// HostOptions renders the run's host options for docker.Run.
func (rm *RunMap) HostOptions() (docker.RunHostOptions, func() error, error) {
	var stack cleanup.Stack

	cwd := rm.cfg.ProjectDir()
	binds, clean, err := rm.binds()
	stack.Push("settle shared agents doc", clean)
	if err != nil {
		return docker.RunHostOptions{}, stack.Run, err
	}

	return docker.RunHostOptions{
		Cwd:                cwd,
		WorkspaceMountPath: workspaceMountPath(cwd),
		Mounts:             binds,
		TmpfsPaths:         tmpfsMasks(cwd, rm.cfg.TmpfsMasksPresent()),
	}, stack.Run, nil
}

// Env renders the container's env in one map: the CLI's fixed env under the config's
// overrides, plus the CLI's pkginfo and the host's GitHub token.
func (rm *RunMap) Env() (map[string]string, error) {
	pkgInfo, err := rm.cli.PkgInfoJSON()
	if err != nil {
		return nil, err
	}
	return mergeempty.Map(mergeempty.Map(rm.cli.Env, rm.cfg.Env), map[string]string{
		pkginfo.EnvVar: pkgInfo,
		envGHToken:     os.Getenv(envGHToken),
	}), nil
}

// Cmd maps the run flags to the CLI's session launch argv.
func (rm *RunMap) Cmd(shell, cont, resume bool, args []string) []string {
	return rm.cli.SessionCmd(shell, cont, resume, args)
}
