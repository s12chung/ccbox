package prompt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/util/osutil"
)

func TestLoadConfirm(t *testing.T) {
	t.Run("missing file is nothing confirmed", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())

		c, err := loadConfirm()
		require.NoError(t, err)
		assert.Empty(t, c.ProjectDirs)
	})

	t.Run("reads the recorded dirs", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		var recorded confirm
		require.NoError(t, recorded.saveProjectDir("/repo/a"))
		require.NoError(t, recorded.saveProjectDir("/repo/b"))

		c, err := loadConfirm()
		require.NoError(t, err)
		assert.Equal(t, []string{"/repo/a", "/repo/b"}, c.ProjectDirs)
	})

	t.Run("broken json errors", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		require.NoError(t, os.MkdirAll(filepath.Dir(confirmPath()), osutil.Dir))
		require.NoError(t, os.WriteFile(confirmPath(), []byte("{"), osutil.File))

		_, err := loadConfirm()
		require.Error(t, err)
	})
}
