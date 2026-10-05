package userdir

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/ccboxtools/pkg/util/testutil"
)

func TestMustHome(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, home, MustHome())

	testutil.NoHome(t)
	assert.Panics(t, func() { MustHome() })
}

func TestRuns(t *testing.T) {
	testutil.FakeHome(t, "/home/me")
	assert.Equal(t, "/home/me/.ccbox/tmp/runs", Runs())
}

func TestTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	assert.Equal(t, "~/foo", Tilde(filepath.Join(home, "foo")))
	assert.Equal(t, "~", Tilde(home))
	assert.Equal(t, filepath.Join(home+"x", "foo"), Tilde(filepath.Join(home+"x", "foo")), "a prefix-like dir is not home")
	assert.Equal(t, "/usr/local", Tilde("/usr/local"))
}
