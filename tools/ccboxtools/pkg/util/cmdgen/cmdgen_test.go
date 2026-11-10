package cmdgen

import (
	"context"
	"os"
	"os/exec"
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

func TestSyscallCmd_Run_ErrorsWithoutBin(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no bin on PATH

	err := SyscallCmd{Bin: "ccbox-definitely-not-a-binary"}.Run()

	require.ErrorIs(t, err, exec.ErrNotFound)
}
