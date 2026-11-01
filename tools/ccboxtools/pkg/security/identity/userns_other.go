//go:build !linux

package identity

import (
	"context"
	"errors"
)

// unshareUserNS refuses on non-linux builds: user namespaces are a linux
// kernel feature.
func unshareUserNS(context.Context) (bool, error) {
	return false, errors.New("unshare requires linux")
}
