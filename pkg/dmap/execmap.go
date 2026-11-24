package dmap

import (
	"strings"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/models/cli"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/proxy"
)

// ExecMode is the exec session's mode: a shell's argv, or the CLI's own launch
type ExecMode struct {
	Shell bool // true execs Args (bare: bash); false launches the CLI's cmd
	Args  []string
}

// ExecMap maps the exec session's mode to the docker pkg exec options
type ExecMap struct {
	cli cli.CLI
}

// NewExecMap returns a new ExecMap
func NewExecMap(c cli.CLI) *ExecMap { return &ExecMap{cli: c} }

// Options renders the exec session's docker.ExecOptions; a plain run skips the
// wrapper chain
func (em *ExecMap) Options(runOptions docker.RunOptions, mode ExecMode, isProxied bool) docker.ExecOptions {
	return docker.ExecOptions{RunOptions: runOptions, Cmd: em.cmd(mode, isProxied)}
}

func (em *ExecMap) cmd(mode ExecMode, isProxied bool) []string {
	var argv []string
	if mode.Shell {
		argv = mode.Args
		if len(argv) == 0 {
			argv = []string{"bash"}
		}
	} else {
		argv = strings.Fields(em.cli.Cmd)
	}
	if !isProxied {
		return append([]string{"ccboxtools", "exec", "--"}, argv...)
	}
	// the container user is root, so the wrapper chain does the session's drop
	return append(proxy.DropChain("ccboxtools", "proxy", "exec", "--"), argv...)
}
