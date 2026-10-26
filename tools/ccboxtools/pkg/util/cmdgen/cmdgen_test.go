package cmdgen

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStdio(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	cmd := Stdio(ctx, "sh", "-c", "exit 0")

	assert.Equal(t, "sh", filepath.Base(cmd.Path))
	assert.Equal(t, []string{"sh", "-c", "exit 0"}, cmd.Args)
	assert.Same(t, os.Stdin, cmd.Stdin)
	assert.Same(t, os.Stdout, cmd.Stdout)
	assert.Same(t, os.Stderr, cmd.Stderr)
}

func TestAsUser(t *testing.T) {
	cmd, err := AsUser(Stdio(t.Context(), "sh"), "root") // the one user every unix bakes
	require.NoError(t, err)

	cred := cmd.SysProcAttr.Credential
	require.NotNil(t, cred)
	assert.Zero(t, cred.Uid)
	assert.Zero(t, cred.Gid)
}

func TestAsUser_UnknownUser(t *testing.T) {
	_, err := AsUser(Stdio(t.Context(), "sh"), "no-such-user")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "user no-such-user")
}
