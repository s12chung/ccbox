package entrypoint

import (
	"errors"
	"fmt"
	"os/user"
)

// checkIdentity asserts the unprivileged container identity: the ccbox user, never
// root, and no sudo to escalate with. lookPath resolves commands like the shell's
// `command -v` this replaces.
func checkIdentity(uid int, name string, lookPath func(string) (string, error)) error {
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

// currentUserName resolves the current user's name; "" on failure, which
// checkIdentity fails closed on.
func currentUserName() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	return u.Username
}
