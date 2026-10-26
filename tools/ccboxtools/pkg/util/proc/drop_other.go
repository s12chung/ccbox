//go:build !linux

package proc

import "errors"

// DropTo refuses the drop: it is linux-only, like the container itself; a
// non-linux build refuses rather than pretend.
func DropTo(int) error {
	return errors.New("the identity drop requires linux")
}

// ReadStatus refuses the status read: it is linux-only, like the container
// itself; a non-linux build refuses rather than pretend.
func ReadStatus() (*Status, error) {
	return nil, errors.New("the status read requires linux")
}
