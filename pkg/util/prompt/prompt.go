// Package prompt has interactive terminal helpers
package prompt

import (
	"bufio"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/moby/term"

	"github.com/s12chung/ccbox/pkg/util/log"
)

// Confirm prints question on stdout and returns true only on y/yes.
func Confirm(question string) bool {
	log.Info(question + " [y/N] ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

// RawTerminal puts stdin into raw mode so keystrokes reach the container
// unbuffered, returning a restore func to undo it. ok is false when stdin isn't
// a terminal (piped/redirected) or raw mode can't be set — then restore is nil
// and there's nothing to undo.
func RawTerminal() (restore func() error, ok bool) {
	inFd, _ := term.GetFdInfo(os.Stdin)
	if !term.IsTerminal(inFd) {
		return nil, false
	}
	state, err := term.SetRawTerminal(inFd)
	if err != nil {
		return nil, false
	}
	return func() error { return term.RestoreTerminal(inFd, state) }, true
}

// ForwardResizes calls onResize with stdout's current window size immediately
// and again on every SIGWINCH, so a consumer (e.g. a container tty) can track
// the terminal's size. The returned stop func ends forwarding. When stdout
// isn't a terminal, sizes can't be read and onResize never fires.
func ForwardResizes(onResize func(h, w uint)) (stop func()) {
	outFd, _ := term.GetFdInfo(os.Stdout)
	emit := func() {
		if ws, err := term.GetWinsize(outFd); err == nil {
			onResize(uint(ws.Height), uint(ws.Width))
		}
	}
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	go func() {
		for range winch {
			emit()
		}
	}()
	emit() // initial size
	return func() { signal.Stop(winch) }
}
