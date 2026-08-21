// Package harness abstracts the coding CLIs ccbox can install and launch
package harness

import (
	"fmt"
	"slices"
	"strings"
)

// Name is the coding CLI's identity — the .ccbox.yaml cli value.
type Name string

const (
	NameClaude   Name = "claude"
	NameCodex    Name = "codex"
	NameOpenCode Name = "opencode"
)

// All lists every supported CLI, in stable order.
func All() []CLI {
	return []CLI{Claude, Codex, OpenCode}
}

// CLI holds everything ccbox does differently per coding CLI.
type CLI struct {
	Name Name

	// Package is the npm package the image installs
	Package string

	// ConfigHomeMount is the CLI's native default config dir in-container within the $HOME, so the
	// mounted config is found with no CLAUDE_CONFIG_DIR/CODEX_HOME override.
	// Threaded into the build too (CONFIG_DIR arg).
	ConfigHomeMount string

	// SeedSrcFolder locates the embedded seed tree for this CLI's config dir;
	// the host config dir reuses its leaf (e.g. .../codex-config -> codex-config).
	SeedSrcFolder string

	// SeedRenames remaps seed paths (source path -> destination name), e.g. the
	// user-editable *.user.md lands as the CLI's live memory file.
	SeedRenames map[string]string

	// Cmd is the launch argv prefix; ContinueArgs/ResumeArgs extend it for the
	// run flags (-c/--resume) in each CLI's own session syntax.
	Cmd          string
	ContinueArgs string
	ResumeArgs   string

	// AllowDomains are the egress wall domains this CLI talks to.
	AllowDomains []string
}

// Claude is Claude Code (Anthropic).
var Claude = CLI{
	Name:            NameClaude,
	Package:         "@anthropic-ai/claude-code",
	ConfigHomeMount: ".claude",
	SeedSrcFolder:   "claude-config",
	SeedRenames:     map[string]string{"CLAUDE.user.md": "CLAUDE.md"},
	Cmd:             "claude",
	ContinueArgs:    "-c",
	ResumeArgs:      "--resume",
	AllowDomains: []string{
		"platform.claude.com",
		"api.anthropic.com",
		"mcp-proxy.anthropic.com",
		"statsig.anthropic.com",
		"sentry.io",
	},
}

// Codex is Codex (OpenAI).
var Codex = CLI{
	Name:            NameCodex,
	Package:         "@openai/codex",
	ConfigHomeMount: ".codex",
	SeedSrcFolder:   "codex-config",
	SeedRenames:     map[string]string{"AGENTS.user.md": "AGENTS.md"},
	Cmd:             "codex",
	ContinueArgs:    "resume --last",
	ResumeArgs:      "resume",
	AllowDomains: []string{
		"api.openai.com",
		"auth.openai.com",
		"chatgpt.com",
	},
}

// OpenCode is OpenCode (Anomaly).
var OpenCode = CLI{
	Name:            NameOpenCode,
	Package:         "opencode-ai",
	ConfigHomeMount: ".config/opencode",
	SeedSrcFolder:   "opencode-config",
	SeedRenames:     map[string]string{"AGENTS.user.md": "AGENTS.md"},
	Cmd:             "opencode",
	ContinueArgs:    "-c",
	// No picker flag exists; bare --session errors, so -r needs a session id.
	ResumeArgs: "--session",
	AllowDomains: []string{
		"opencode.ai",
		"models.dev",
	},
}

// For looks up the CLI by name. ok is false for an unknown name.
func For(name Name) (CLI, bool) {
	for _, c := range All() {
		if c.Name == name {
			return c, true
		}
	}
	return CLI{}, false
}

// MustFor is For for names already validated (projectcfg.Load rejects unknown
// cli values); it panics on an unknown name.
func MustFor(name Name) CLI {
	c, ok := For(name)
	if !ok {
		panic(fmt.Sprintf("harness: unknown cli %q", name))
	}
	return c
}

// SessionCmd maps the run flags to the CLI's session syntax: continue the last
// session, resume one (bare for the picker, or named via args), or launch fresh.
func (c CLI) SessionCmd(cont, resume bool, args []string) []string {
	cmd := []string{c.Cmd}
	switch {
	case cont:
		cmd = append(cmd, c.ContinueArgs)
	case resume:
		cmd = append(cmd, c.ResumeArgs, strings.Join(args, " "))
	}
	cmd = slices.DeleteFunc(cmd, func(s string) bool {
		return s == ""
	})
	return strings.Split(strings.Join(cmd, " "), " ")
}
