// Package errs holds error helpers.
package errs

import (
	"errors"
	"fmt"
)

// Swallow nils err when it matches any of the targets (errors.Is semantics);
// every other error, nil included, passes through unchanged.
func Swallow(err error, targets ...error) error {
	for _, target := range targets {
		if errors.Is(err, target) {
			return nil
		}
	}
	return err
}

// exitError reports an exit code up through returned errors: a normal, user-facing
// result (a container's exit status) rather than a failure to log.
type exitError struct{ code int }

func (e exitError) Error() string { return fmt.Sprintf("exit code %d", e.code) }

// Exit returns the code as an error, nil for 0 — a clean exit isn't a non-nil error.
func Exit(code int) error {
	if code == 0 {
		return nil
	}
	return exitError{code: code}
}

// ExitCode unwraps err's exit code, reporting whether err carries one.
func ExitCode(err error) (int, bool) {
	if e, ok := errors.AsType[exitError](err); ok {
		return e.code, true
	}
	return 0, false
}
