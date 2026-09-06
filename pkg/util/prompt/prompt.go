// Package prompt has interactive terminal helpers
package prompt

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/moby/term"
)

// RawTerminal puts stdin into raw mode so keystrokes reach the container
// unbuffered, returning a restore func to undo it. ok is false when stdin isn't
// a terminal (piped/redirected) or raw mode can't be set — then restore is nil
// and there's nothing to undo.
func RawTerminal() (func() error, bool) {
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
func ForwardResizes(onResize func(h, w uint)) func() {
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
