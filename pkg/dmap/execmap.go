package dmap

import (
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/models/cli"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/proxy"
)

// ExecMode is the exec session's selected mode, with Args carrying its args
type ExecMode struct {
	Shell    bool     // the exec subcommand: Args exec in the container
	Continue bool     // -c: continue the last session
	Resume   bool     // -r: resume a session
	NoProxy  bool     // --no-proxy: run without the egress wall
	Args     []string // the exec argv (exec), or the resumed session's name (-r)
}

// ExecMap maps the exec session's mode to the docker pkg exec options
type ExecMap struct {
	cli cli.CLI
}

// NewExecMap returns a new ExecMap
func NewExecMap(c cli.CLI) *ExecMap { return &ExecMap{cli: c} }

// Options renders the exec session's docker.ExecOptions; noProxy runs plain
func (em *ExecMap) Options(runOptions docker.RunOptions, mode ExecMode, noProxy bool) docker.ExecOptions {
	return docker.ExecOptions{RunOptions: runOptions, Cmd: em.cmd(mode, noProxy)}
}

// cmd maps the exec mode to the session's argv
func (em *ExecMap) cmd(mode ExecMode, noProxy bool) []string {
	var argv []string
	if mode.Shell {
		argv = mode.Args
		if len(argv) == 0 {
			argv = []string{"bash"}
		}
	} else {
		argv = em.cli.SessionCmd(mode.Continue, mode.Resume, mode.Args)
	}
	if noProxy {
		return append([]string{"ccboxtools", "exec", "--"}, argv...)
	}
	// the container user is root, so the wrapper chain does the session's drop
	return append(proxy.DropChain("ccboxtools", "proxy", "exec", "--"), argv...)
}
