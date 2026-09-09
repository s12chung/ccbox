package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/pkg/harness"
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

func TestAgentsMdBind(t *testing.T) {
	t.Run("binds as the CLI's AGENTS.md file within its config mount", func(t *testing.T) {
		for _, tc := range []struct {
			cli string
			dir string
		}{
			{cli: "claude", dir: ".claude"}, // yaml override: CLAUDE.md
			{cli: "codex", dir: ".codex"},   // parse default: AGENTS.md
			{cli: "opencode", dir: ".config/opencode"},
		} {
			cli := harness.MustFor(tc.cli)
			host := "/host/.ccbox/tmp/" + tc.cli + "/" + cli.SeedAgentsFilename
			got := agentsMdBind(RunOptions{CLI: tc.cli, AgentsMdBind: host})
			assert.Equal(t, []string{host + ":/home/ccbox/" + tc.dir + "/" + cli.SeedAgentsFilename}, got, tc.cli)
		}
	})

	t.Run("no bind when the CLI has its own", func(t *testing.T) {
		assert.Empty(t, agentsMdBind(RunOptions{CLI: "claude"}))
	})
}
