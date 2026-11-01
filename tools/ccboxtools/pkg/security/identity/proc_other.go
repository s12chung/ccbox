//go:build !linux

package identity

import "errors"

func mountProcfs(string, uintptr, string) error {
	return errors.New("procfs remount requires linux")
}
