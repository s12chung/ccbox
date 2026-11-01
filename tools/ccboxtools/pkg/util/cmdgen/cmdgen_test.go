package cmdgen

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
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
