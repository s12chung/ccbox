package prompt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/kit/pick"
	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/size"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

func TestMain(m *testing.M) {
	// point HOME at a temp tree, so confirm.json lands there, not the real ~/.ccbox
	dir := must.Get(os.MkdirTemp("", "ccbox-prompt-test"))
	must.Do(os.Setenv("HOME", dir))
	code := m.Run()
	must.Do(os.RemoveAll(dir))
	os.Exit(code)
}

func TestProjectDirSize(t *testing.T) {
	dir := t.TempDir()

	// stubDirOver pins dirOver's answer for ProjectDirSize
	stubDirOver := func(t *testing.T, over bool) {
		t.Helper()
		dirOver = func(string, int64) (bool, error) { return over, nil }
		t.Cleanup(func() { dirOver = size.DirOver })
	}
	// confirmed reads the recorded dirs
	confirmed := func(t *testing.T) []string {
		t.Helper()
		c, err := loadConfirm()
		require.NoError(t, err)
		return c.ProjectDirs
	}
	// recordDir pre-records dir, like a past run's yes
	recordDir := func(t *testing.T, dir string) {
		t.Helper()
		c, err := loadConfirm()
		require.NoError(t, err)
		require.NoError(t, c.saveProjectDir(dir))
	}

	t.Run("under limit passes quietly", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		stubDirOver(t, false)
		testutil.Stdin(t, "n\n") // a prompt would read this no and abort

		require.NoError(t, ProjectDirSize(dir))
		assert.Empty(t, confirmed(t))
	})

	t.Run("yes is recorded", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		stubDirOver(t, true)
		testutil.Stdin(t, "y\n")

		require.NoError(t, ProjectDirSize(dir))
		assert.Equal(t, []string{dir}, confirmed(t))
	})

	t.Run("recorded yes passes quietly", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		stubDirOver(t, true)
		recordDir(t, dir)
		testutil.Stdin(t, "n\n") // a prompt would read this no and abort

		require.NoError(t, ProjectDirSize(dir))
	})

	t.Run("no aborts", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		stubDirOver(t, true)
		testutil.Stdin(t, "n\n")

		err := ProjectDirSize(dir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "wasn't confirmed")
		assert.Contains(t, err.Error(), filepath.Base(confirmPath()))
	})

	t.Run("eof reads as no", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		stubDirOver(t, true)
		testutil.Stdin(t, "")

		require.Error(t, ProjectDirSize(dir))
	})

	t.Run("broken confirm.json errors", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		stubDirOver(t, true)
		configDir := filepath.Join(home, ".ccbox", "config")
		require.NoError(t, os.MkdirAll(configDir, osutil.Dir))
		require.NoError(t, os.WriteFile(filepath.Join(configDir, "confirm.json"), []byte("{"), osutil.File))

		require.Error(t, ProjectDirSize(dir))
	})

	t.Run("sizing error propagates", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		dirOver = func(string, int64) (bool, error) { return false, os.ErrPermission }
		t.Cleanup(func() { dirOver = size.DirOver })

		require.ErrorIs(t, ProjectDirSize(dir), os.ErrPermission)
	})
}

func TestHarnessCLI(t *testing.T) {
	// stubSelect pins selectFn's answer
	stubSelect := func(t *testing.T, name string, err error) {
		t.Helper()
		selectFn = func(string, []string, []string) (string, error) { return name, err }
		t.Cleanup(func() { selectFn = pick.Select })
	}

	t.Run("picked name is returned", func(t *testing.T) {
		stubSelect(t, "pi", nil)

		name, err := HarnessCLI([]string{"claude", "pi"})
		require.NoError(t, err)
		assert.Equal(t, "pi", name)
	})

	t.Run("no picker answer errors", func(t *testing.T) {
		stubSelect(t, "", nil)

		_, err := HarnessCLI([]string{"claude", "pi"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no harness CLI selected")
	})
}
