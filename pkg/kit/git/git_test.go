package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestXDGConfigDir(t *testing.T) {
	t.Run("XDG_CONFIG_HOME set", func(t *testing.T) {
		base := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", base)

		assert.Equal(t, base+"/git", XDGConfigDir())
	})

	t.Run("XDG_CONFIG_HOME unset falls back to ~/.config/git", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", "") // empty is treated as unset
		t.Setenv("HOME", home)          // MustHome reads $HOME

		assert.Equal(t, home+"/.config/git", XDGConfigDir())
	})

	t.Run("unresolvable home panics", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "")

		assert.Panics(t, func() { XDGConfigDir() })
	})
}
