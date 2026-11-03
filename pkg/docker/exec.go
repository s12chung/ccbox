package docker

import (
	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// ExecOptions is the exec session's workload for the devbox's entrypoint to run.
type ExecOptions struct {
	RunOptions RunOptions // the devbox the session execs in

	Cmd []string // the session's exec argv, under its wrapper
}

// Exec boots into the devbox, then execs the session wired to the local terminal
func Exec(ctxD *dock.CtxD, o ExecOptions) error {
	leave, err := holdDevbox(ctxD, o.RunOptions)
	if err != nil {
		return err
	}
	defer log.Defer("devbox leave", leave)
	return dock.ExecInteractive(ctxD, o.RunOptions.Name, o.Cmd)
}
