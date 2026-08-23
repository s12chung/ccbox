package harness

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllAlphaOrder(t *testing.T) {
	names := make([]string, 0, len(all))
	for _, c := range all {
		names = append(names, c.Name)
	}
	assert.Equal(t, []string{"claude", "codex", "grok", "opencode"}, names)
}

func TestFor(t *testing.T) {
	for _, want := range All() {
		got, ok := For(want.Name)
		assert.True(t, ok, want.Name)
		assert.Equal(t, want, got)
	}

	got, ok := For("emacs")
	assert.False(t, ok, "unknown name")
	assert.Zero(t, got)
}

func TestMustFor(t *testing.T) {
	for _, want := range All() {
		assert.Equal(t, want, MustFor(want.Name))
	}
	assert.PanicsWithValue(t, `harness: unknown cli "emacs"`, func() { MustFor("emacs") })
}

func TestPkgers(t *testing.T) {
	// the PKGER build arg each CLI's install source renders
	assert.Equal(t, "npm:@anthropic-ai/claude-code", MustFor("claude").Pkger().Arg())
	assert.Equal(t, "npm:@openai/codex", MustFor("codex").Pkger().Arg())
	assert.Equal(t, "npm:opencode-ai", MustFor("opencode").Pkger().Arg())
	assert.Equal(t,
		"versionurl:https://x.ai/cli/stable|https://x.ai/cli/grok-$version-linux-x86_64|https://x.ai/cli/grok-$version-linux-aarch64",
		MustFor("grok").Pkger().Arg())
}

func TestSeedAgentsFilename(t *testing.T) {
	assert.Equal(t, "CLAUDE.md", MustFor("claude").SeedAgentsFilename) // yaml override
	assert.Equal(t, "AGENTS.md", MustFor("codex").SeedAgentsFilename)  // parse default
}

func TestEnv(t *testing.T) {
	// Config-dir overrides point at the CLI's native config mount; toggles disable
	// update checks / nonessential traffic. pkg/docker merges these into every run.
	assert.Equal(t, "CLAUDE_CONFIG_DIR", MustFor("claude").ConfigDirEnvKey)
	assert.Equal(t, map[string]string{
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
		"DISABLE_AUTOUPDATER":                      "1",
	}, MustFor("claude").Env)

	assert.Equal(t, "CODEX_HOME", MustFor("codex").ConfigDirEnvKey)
	assert.Empty(t, MustFor("codex").Env)

	assert.Empty(t, MustFor("opencode").ConfigDirEnvKey)
	assert.Equal(t, map[string]string{"OPENCODE_DISABLE_AUTOUPDATE": "1"}, MustFor("opencode").Env)

	assert.Empty(t, MustFor("grok").ConfigDirEnvKey)
	assert.Equal(t, map[string]string{"GROK_DISABLE_AUTOUPDATER": "1"}, MustFor("grok").Env)
}

func TestSessionCmd(t *testing.T) {
	tests := []struct {
		cli    CLI
		shell  bool
		cont   bool
		resume bool
		args   []string
		want   []string
	}{
		{cli: MustFor("claude"), shell: true, want: nil},
		{cli: MustFor("claude"), want: []string{"claude"}},
		{cli: MustFor("claude"), cont: true, want: []string{"claude", "-c"}},
		{cli: MustFor("claude"), resume: true, want: []string{"claude", "--resume"}},
		{cli: MustFor("claude"), resume: true, args: []string{"auth-refactor"}, want: []string{"claude", "--resume", "auth-refactor"}},

		{cli: MustFor("codex"), want: []string{"codex", "--sandbox", "danger-full-access"}},
		{cli: MustFor("codex"), cont: true, want: []string{"codex", "--sandbox", "danger-full-access", "resume", "--last"}},
		{cli: MustFor("codex"), resume: true, want: []string{"codex", "--sandbox", "danger-full-access", "resume"}},
		{cli: MustFor("codex"), resume: true, args: []string{"abc123"}, want: []string{"codex", "--sandbox", "danger-full-access", "resume", "abc123"}},

		{cli: MustFor("opencode"), want: []string{"opencode"}},
		{cli: MustFor("opencode"), cont: true, want: []string{"opencode", "-c"}},
		// No picker flag: bare --session errors in opencode, so -r needs an id
		{cli: MustFor("opencode"), resume: true, want: []string{"opencode", "--session"}},
		{cli: MustFor("opencode"), resume: true, args: []string{"ses_42"}, want: []string{"opencode", "--session", "ses_42"}},

		{cli: MustFor("grok"), want: []string{"grok"}},
		{cli: MustFor("grok"), cont: true, want: []string{"grok", "-c"}},
		// Bare --resume resumes the most recent session
		{cli: MustFor("grok"), resume: true, want: []string{"grok", "--resume"}},
		{cli: MustFor("grok"), resume: true, args: []string{"abc-uuid"}, want: []string{"grok", "--resume", "abc-uuid"}},
	}
	for _, tt := range tests {
		got := tt.cli.SessionCmd(tt.shell, tt.cont, tt.resume, tt.args)
		assert.Equal(t, tt.want, got, "%s shell=%v cont=%v resume=%v", tt.cli.Name, tt.shell, tt.cont, tt.resume)
	}
}
