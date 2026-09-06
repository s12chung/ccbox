package userdir

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	assert.Equal(t, "~/foo", Tilde(filepath.Join(home, "foo")))
	assert.Equal(t, "~", Tilde(home))
	assert.Equal(t, filepath.Join(home+"x", "foo"), Tilde(filepath.Join(home+"x", "foo")), "a prefix-like dir is not home")
	assert.Equal(t, "/usr/local", Tilde("/usr/local"))
}
