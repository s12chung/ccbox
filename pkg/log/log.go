// Package log writes plain user-facing lines: status to stdout, problems to stderr.
package log

import (
	"fmt"
	"io"
	"os"
)

var ( // swapped in tests
	outInfo io.Writer = os.Stdout
	outErr  io.Writer = os.Stderr
)

func Info(msg string)                { fmt.Fprintln(outInfo, msg) }
func Infof(format string, a ...any)  { fmt.Fprintln(outInfo, fmt.Sprintf(format, a...)) }
func Warnf(format string, a ...any)  { fmt.Fprintln(outErr, fmt.Sprintf(format, a...)) }
func Errorf(format string, a ...any) { fmt.Fprintln(outErr, fmt.Sprintf(format, a...)) }

// Defer runs a deferred cleanup fn and surfaces (rather than swallows) its error.
func Defer(what string, fn func() error) {
	if err := fn(); err != nil {
		Warnf("%s failed: %v", what, err)
	}
}
