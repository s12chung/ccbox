package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

func TestXDGConfigDir(t *testing.T) {
	// The two settings differ only in where the config base comes from; the git/ entry sits under it.
	settings := []struct {
		name string
		base func(t *testing.T) string // sets env, returns the dir that should contain the git/ entry
	}{
		{"XDG_CONFIG_HOME set", func(t *testing.T) string {
			base := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", base)
			return base
		}},
		{"XDG_CONFIG_HOME unset falls back to ~/.config", func(t *testing.T) string {
			home := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", "") // empty is treated as unset
			t.Setenv("HOME", home)          // os.UserHomeDir reads $HOME
			return filepath.Join(home, ".config")
		}},
	}

	// Each case places (or omits) the git/ entry under base and returns the expected result ("" = skip).
	cases := []struct {
		name  string
		setup func(t *testing.T, base string) string
	}{
		{"present dir is returned", func(t *testing.T, base string) string {
			dir := filepath.Join(base, "git")
			require.NoError(t, os.MkdirAll(dir, ioutil.Dir))
			return dir
		}},
		{"absent yields empty", func(_ *testing.T, _ string) string {
			return ""
		}},
		{"a file is skipped", func(t *testing.T, base string) string {
			require.NoError(t, os.MkdirAll(base, ioutil.Dir))
			require.NoError(t, os.WriteFile(filepath.Join(base, "git"), nil, ioutil.File)) // file, not dir
			return ""
		}},
	}

	for _, s := range settings {
		for _, c := range cases {
			t.Run(s.name+"/"+c.name, func(t *testing.T) {
				want := c.setup(t, s.base(t))
				got, err := XDGConfigDir()
				require.NoError(t, err)
				assert.Equal(t, want, got)
			})
		}
	}
}

func TestMustXDGConfigDir(t *testing.T) {
	t.Run("returns the dir", func(t *testing.T) {
		base := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", base)
		require.NoError(t, os.MkdirAll(filepath.Join(base, "git"), ioutil.Dir))

		assert.Equal(t, filepath.Join(base, "git"), MustXDGConfigDir())
	})

	t.Run("panics on error", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "") // os.UserHomeDir errors on an empty $HOME

		assert.PanicsWithError(t, "$HOME is not defined", func() { MustXDGConfigDir() })
	})
}
