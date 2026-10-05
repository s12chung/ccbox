package git

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/ccboxtools/pkg/util/testutil"
)

func TestXDGConfigDir(t *testing.T) {
	t.Run("XDG_CONFIG_HOME set", func(t *testing.T) {
		base := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", base)

		assert.Equal(t, base+"/git", XDGConfigDir())
	})

	t.Run("XDG_CONFIG_HOME unset falls back to ~/.config/git", func(t *testing.T) {
		home := testutil.Home(t)

		assert.Equal(t, home+"/.config/git", XDGConfigDir())
	})

	t.Run("unresolvable home panics", func(t *testing.T) {
		testutil.NoHome(t)

		assert.Panics(t, func() { XDGConfigDir() })
	})
}
