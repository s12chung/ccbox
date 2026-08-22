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

// Info prints msg to stdout.
func Info(msg string) { _, _ = fmt.Fprintln(outInfo, msg) }

// Infof prints the formatted msg to stdout.
func Infof(format string, a ...any) { _, _ = fmt.Fprintln(outInfo, fmt.Sprintf(format, a...)) }

// Warnf prints the formatted msg to stderr.
func Warnf(format string, a ...any) { _, _ = fmt.Fprintln(outErr, fmt.Sprintf(format, a...)) }

// Errorf prints the formatted msg to stderr.
func Errorf(format string, a ...any) { _, _ = fmt.Fprintln(outErr, fmt.Sprintf(format, a...)) }

// Defer runs a deferred cleanup fn and surfaces (rather than swallows) its error.
func Defer(what string, fn func() error) { WarnErr(what, fn()) }

// WarnErr logs err against what when non-nil.
func WarnErr(what string, err error) {
	if err != nil {
		Warnf("%s failed: %v", what, err)
	}
}
