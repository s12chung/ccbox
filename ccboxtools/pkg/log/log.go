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

// Infof prints the formatted msg to stdout.
func Infof(format string, a ...any) { _, _ = fmt.Fprintln(outInfo, fmt.Sprintf(format, a...)) }

// Warnf prints the formatted msg to stderr.
func Warnf(format string, a ...any) { _, _ = fmt.Fprintln(outErr, fmt.Sprintf(format, a...)) }

// Errorf prints the formatted msg to stderr.
func Errorf(format string, a ...any) { _, _ = fmt.Fprintln(outErr, fmt.Sprintf(format, a...)) }
