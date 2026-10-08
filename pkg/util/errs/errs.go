// Package errs swallows sentinel errors
package errs

import "errors"

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
