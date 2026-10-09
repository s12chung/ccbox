// Package term has interactive terminal helpers
package term

import (
	"bufio"
	"os"
	"os/signal"
	"strings"
	"syscall"

	mobyterm "github.com/moby/term"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// Confirm prints question on stdout and returns true only on y/yes. Piped or closed
// stdin reads EOF and answers no.
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

// Raw puts stdin into raw mode so keystrokes reach the container
// unbuffered, returning a restore func to undo it. ok is false when stdin isn't
// a terminal (piped/redirected) or raw mode can't be set — then restore is nil
// and there's nothing to undo.
func Raw() (func() error, bool) {
	inFd, _ := mobyterm.GetFdInfo(os.Stdin)
	if !mobyterm.IsTerminal(inFd) {
		return nil, false
	}
	state, err := mobyterm.SetRawTerminal(inFd)
	if err != nil {
		return nil, false
	}
	return func() error { return mobyterm.RestoreTerminal(inFd, state) }, true
}

// ForwardResizes calls onResize with stdout's current window size immediately
// and again on every SIGWINCH, so a consumer (e.g. a container tty) can track
// the terminal's size. The returned stop func ends forwarding. When stdout
// isn't a terminal, sizes can't be read and onResize never fires.
func ForwardResizes(onResize func(h, w uint)) func() {
	outFd, _ := mobyterm.GetFdInfo(os.Stdout)
	emit := func() {
		if ws, err := mobyterm.GetWinsize(outFd); err == nil {
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
