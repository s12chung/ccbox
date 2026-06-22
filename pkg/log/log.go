// Package log surfaces deferred cleanup errors that would otherwise be swallowed.
package log

import "log/slog"

// Defer runs a deferred cleanup fn and logs (rather than swallows) its error.
func Defer(what string, fn func() error) {
	if err := fn(); err != nil {
		slog.Warn(what+" failed", "error", err)
	}
}
