package harness

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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

func TestSessionCmd(t *testing.T) {
	tests := []struct {
		cli    CLI
		cont   bool
		resume bool
		args   []string
		want   []string
	}{
		{cli: Claude, want: []string{"claude"}},
		{cli: Claude, cont: true, want: []string{"claude", "-c"}},
		{cli: Claude, resume: true, want: []string{"claude", "--resume"}},
		{cli: Claude, resume: true, args: []string{"auth-refactor"}, want: []string{"claude", "--resume", "auth-refactor"}},

		{cli: Codex, want: []string{"codex"}},
		{cli: Codex, cont: true, want: []string{"codex", "resume", "--last"}},
		{cli: Codex, resume: true, want: []string{"codex", "resume"}},
		{cli: Codex, resume: true, args: []string{"abc123"}, want: []string{"codex", "resume", "abc123"}},

		{cli: OpenCode, want: []string{"opencode"}},
		{cli: OpenCode, cont: true, want: []string{"opencode", "-c"}},
		// No picker flag: bare --session errors in opencode, so -r needs an id
		{cli: OpenCode, resume: true, want: []string{"opencode", "--session"}},
		{cli: OpenCode, resume: true, args: []string{"ses_42"}, want: []string{"opencode", "--session", "ses_42"}},

		{cli: Grok, want: []string{"grok"}},
		{cli: Grok, cont: true, want: []string{"grok", "-c"}},
		// Bare --resume resumes the most recent session
		{cli: Grok, resume: true, want: []string{"grok", "--resume"}},
		{cli: Grok, resume: true, args: []string{"abc-uuid"}, want: []string{"grok", "--resume", "abc-uuid"}},
	}
	for _, tt := range tests {
		got := tt.cli.SessionCmd(tt.cont, tt.resume, tt.args)
		assert.Equal(t, tt.want, got, "%s cont=%v resume=%v", tt.cli.Name, tt.cont, tt.resume)
	}
}
