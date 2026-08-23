package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvStringBaseWins(t *testing.T) {
	got := envString(RunOptions{
		CLI:     "claude",
		GHToken: "gh",
		Env:     map[string]string{"GOFLAGS": "-mod=mod", "http_proxy": "evil"},
	})

	// CLI defaults, then o.Env sorted, then the base vars last — so the base http_proxy is the
	// final (winning) value.
	assert.Equal(t, []string{
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1",
		"DISABLE_AUTOUPDATER=1",
		"GOFLAGS=-mod=mod",
		"http_proxy=evil",
		"http_proxy=http://ccbox-egress:8888",
		"https_proxy=http://ccbox-egress:8888",
		"no_proxy=localhost,127.0.0.1,::1",
		"NO_PROXY=localhost,127.0.0.1,::1",
		"GH_TOKEN=gh",
	}, got)
}

func TestEnvStringNoProxy(t *testing.T) {
	got := envString(RunOptions{
		CLI:     "claude",
		GHToken: "gh",
		NoProxy: true,
	})

	// No wall, no proxy vars — just the CLI defaults and the token.
	assert.Equal(t, []string{
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1",
		"DISABLE_AUTOUPDATER=1",
		"GH_TOKEN=gh",
	}, got)
}

func TestEnvStringCLIDefaults(t *testing.T) {
	tests := []struct {
		cli  string
		want []string
	}{
		{
			cli: "claude",
			want: []string{
				"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1",
				"DISABLE_AUTOUPDATER=1",
			},
		},
		{cli: "codex", want: []string{}},
		{cli: "opencode", want: []string{"OPENCODE_DISABLE_AUTOUPDATE=1"}},
		{cli: "grok", want: []string{"GROK_DISABLE_AUTOUPDATER=1"}},
	}
	for _, tt := range tests {
		got := envString(RunOptions{CLI: tt.cli})
		require.Len(t, got, len(tt.want)+5, tt.cli) // + the base vars
		assert.Equal(t, tt.want, got[:len(tt.want)], tt.cli)
	}
}

func TestEnvStringUnknownCLIPanics(t *testing.T) {
	assert.PanicsWithValue(t, `harness: unknown cli "emacs"`, func() {
		envString(RunOptions{CLI: "emacs"})
	})
}

func TestEnvStringUserOverridesCLI(t *testing.T) {
	got := envString(RunOptions{
		CLI: "opencode",
		Env: map[string]string{"OPENCODE_DISABLE_AUTOUPDATE": "0"},
	})

	// Both entries are emitted; Docker takes the last (user's) value.
	assert.Equal(t, []string{
		"OPENCODE_DISABLE_AUTOUPDATE=1",
		"OPENCODE_DISABLE_AUTOUPDATE=0",
	}, got[:2])
}
