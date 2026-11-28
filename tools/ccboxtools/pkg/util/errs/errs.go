// Package errs holds error helpers.
package errs

import "fmt"

// Prefix wraps err with prefix — nil passes through.
func Prefix(prefix string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", prefix, err)
}
