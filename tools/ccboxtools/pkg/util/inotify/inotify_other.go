//go:build !linux

package inotify

import (
	"context"
	"errors"
)

// errRefused refuses the watch: inotify is linux-only, like the container
// itself; a non-linux build refuses rather than pretend.
var errRefused = errors.New("inotify requires linux")

// watch refuses the watch.
func watch(context.Context, string, func(string)) error {
	return errRefused
}
