package vnc

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkger/artifact"
)

func TestStartDesktop_Missing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	err := StartDesktop()

	require.Error(t, err)
	assert.ErrorIs(t, err, exec.ErrNotFound)
}

func TestStartDesktop_StartFails(t *testing.T) {
	// the script's interpreter is missing — LookPath finds it, the exec doesn't
	dir := t.TempDir()
	script := filepath.Join(dir, desktopBin)
	require.NoError(t, os.WriteFile(script, []byte("#!/nonexistent-interpreter\n"), artifact.ExecFileMode))
	t.Setenv("PATH", dir)

	err := StartDesktop()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "vnc requested")
}
