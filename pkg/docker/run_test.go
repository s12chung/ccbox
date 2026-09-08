package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCLIDataBinds(t *testing.T) {
	t.Run("renders rw binds under containerHome, sorted for a deterministic spec", func(t *testing.T) {
		got := cliDataBinds(map[string]string{
			"/host/data/opencode/.local-share-opencode-auth.json": ".local/share/opencode/auth.json",
			"/host/data/opencode/.config-opencode":                ".config/opencode",
		})
		assert.Equal(t, []string{
			"/host/data/opencode/.config-opencode:/home/ccbox/.config/opencode",
			"/host/data/opencode/.local-share-opencode-auth.json:/home/ccbox/.local/share/opencode/auth.json",
		}, got)
	})

	t.Run("empty map renders no binds", func(t *testing.T) {
		assert.Empty(t, cliDataBinds(nil))
	})
}
