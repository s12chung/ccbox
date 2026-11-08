package docker

import (
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/util/errs"
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

	// On a kill/hangup, leave stops the devbox when this is the last holder —
	// an unclean session end can't orphan the boot; the stop EOFs the attach
	// below. Raw-mode Ctrl-C already goes to the container, so SIGINT here
	// only fires for non-tty stdin / `kill -INT`.
	kill := make(chan os.Signal, 1)
	signal.Notify(kill, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(kill)
	go func() {
		<-kill
		log.WarnErr("devbox leave", leave())
	}()
	defer log.Defer("devbox leave", leave)

	return errs.Swallow(dock.ExecInteractive(ctxD, o.RunOptions.Name, o.Cmd), net.ErrClosed, os.ErrClosed, io.ErrClosedPipe)
}
