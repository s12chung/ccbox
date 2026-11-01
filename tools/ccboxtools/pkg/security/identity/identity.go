// Package identity is the workload's unprivileged identity: the boot's arrival
// proof — the ccbox user, never root, with no sudo to escalate with.
package identity

import (
	"errors"
	"fmt"
	"os/user"
)

// The unprivileged container identity, matching the image's useradd.
const (
	ccboxUID  = 1000
	ccboxUser = "ccbox"
)

// CheckIdentity asserts the unprivileged container identity: the ccbox user, never
// root, and no sudo to escalate with. lookPath resolves commands like the shell's
// `command -v` this replaces.
func CheckIdentity(uid int, name string, lookPath func(string) (string, error)) error {
	if uid != ccboxUID {
		return fmt.Errorf("not uid %d (got %d)", ccboxUID, uid)
	}
	if name != ccboxUser {
		return fmt.Errorf("user is not %s (got %s)", ccboxUser, name)
	}
	if _, err := lookPath("sudo"); err == nil {
		return errors.New("sudo is present")
	}
	return nil
}

// CurrentUserName resolves the current user's name; "" on failure, which
// CheckIdentity fails closed on.
func CurrentUserName() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	return u.Username
}
