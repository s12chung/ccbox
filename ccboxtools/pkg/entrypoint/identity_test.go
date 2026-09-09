package entrypoint

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckIdentity(t *testing.T) {
	noSudo := func(string) (string, error) { return "", exec.ErrNotFound }
	sudo := func(string) (string, error) { return "/usr/bin/sudo", nil }

	tests := []struct {
		name     string
		uid      int
		user     string
		lookPath func(string) (string, error)
		wantErr  string
	}{
		{"ok", ccboxUID, ccboxUser, noSudo, ""},
		{"root", 0, ccboxUser, noSudo, "not uid 1000 (got 0)"},
		{"not ccbox", ccboxUID, "root", noSudo, "user is not ccbox"},
		{"sudo present", ccboxUID, ccboxUser, sudo, "sudo is present"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkIdentity(tt.uid, tt.user, tt.lookPath)

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
