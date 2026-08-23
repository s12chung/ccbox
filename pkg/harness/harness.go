// Package harness abstracts the coding CLIs ccbox can install and launch
package harness

import (
	"embed"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/s12chung/ccbox/pkg/kit/pkger"
)

// Name is the coding CLI's identity — the .ccbox.yaml cli value.
type Name string

// Supported coding CLIs.
const (
	NameClaude   Name = "claude"
	NameCodex    Name = "codex"
	NameOpenCode Name = "opencode"
	NameGrok     Name = "grok"
)

// SharedSeedPath is the seed subtree shared across every CLI.
const SharedSeedPath = "shared"

// SeedConfigDir is the per-CLI subdirectory holding the CLI's own seed tree.
const SeedConfigDir = "config"

// agentsMD is the live memory filename most CLIs rename the shared doc into.
const agentsMD = "AGENTS.md"

// AgentsFileName is the shared user AGENTS.md
const AgentsFileName = "AGENTS.user.md"

//go:embed clis
var clisFS embed.FS

// SeedFS returns the embedded seed trees (shared/, per-CLI), rooted at their
// common parent.
func SeedFS() fs.FS {
	sub, err := fs.Sub(clisFS, "clis")
	if err != nil {
		panic(err) // unreachable: the //go:embed pattern above guarantees clis exists
	}
	return sub
}

// All lists every supported CLI, in stable order.
func All() []CLI {
	return []CLI{Claude, Codex, OpenCode, Grok}
}

// CLI holds everything ccbox does differently per coding CLI.
type CLI struct {
	Name Name

	// Pkger locates the CLI's install source
	Pkger pkger.Pkger

	// ConfigHomeMount is the CLI's native default config dir in-container within the $HOME, so the
	// mounted config is found with no override; ConfigDirEnvKey points the CLI's env var at it.
	ConfigHomeMount string

	// ConfigDirEnvKey is the env var naming the mounted config path for this CLI
	// (e.g. CLAUDE_CONFIG_DIR), set per run by pkg/docker; "" = none.
	ConfigDirEnvKey string

	// Env is fixed container env the CLI requires (updater/traffic toggles), merged into
	// every run of this CLI.
	Env map[string]string

	// SeedAgentsFilename is the destination name of the shared all/ memory doc
	// (AgentsFileName) in the host config dir — this CLI's live memory file.
	SeedAgentsFilename string

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
	Pkger:           pkger.Npm{Package: "@anthropic-ai/claude-code"},
	ConfigHomeMount: ".claude",
	ConfigDirEnvKey: "CLAUDE_CONFIG_DIR",
	Env: map[string]string{
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
		"DISABLE_AUTOUPDATER":                      "1",
	},
	SeedAgentsFilename: "CLAUDE.md",
	Cmd:                "claude",
	ContinueArgs:       "-c",
	ResumeArgs:         "--resume",
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
	Name:               NameCodex,
	Pkger:              pkger.Npm{Package: "@openai/codex"},
	ConfigHomeMount:    ".codex",
	ConfigDirEnvKey:    "CODEX_HOME",
	SeedAgentsFilename: agentsMD,
	Cmd:                "codex --sandbox danger-full-access", // run without bubblewrap, which is buggy atm without root
	ContinueArgs:       "resume --last",
	ResumeArgs:         "resume",
	AllowDomains: []string{
		"api.openai.com",
		"auth.openai.com",
		"chatgpt.com",
	},
}

// OpenCode is OpenCode (Anomaly).
var OpenCode = CLI{
	Name:               NameOpenCode,
	Pkger:              pkger.Npm{Package: "opencode-ai"},
	ConfigHomeMount:    ".config/opencode",
	SeedAgentsFilename: agentsMD,
	Cmd:                "opencode",
	ContinueArgs:       "-c",
	// No picker flag exists; bare --session errors, so -r needs a session id.
	ResumeArgs: "--session",
	Env:        map[string]string{"OPENCODE_DISABLE_AUTOUPDATE": "1"},
	AllowDomains: []string{
		"opencode.ai",
		"models.dev",
	},
}

// Grok is Grok Build (xAI).
var Grok = CLI{
	Name: NameGrok,
	Pkger: pkger.VersionURL{
		URL:           "https://x.ai/cli/stable",
		LinuxX64URL:   "https://x.ai/cli/grok-$version-linux-x86_64",
		LinuxArm64URL: "https://x.ai/cli/grok-$version-linux-aarch64",
	},
	ConfigHomeMount:    ".grok",
	SeedAgentsFilename: agentsMD,
	Cmd:                "grok",
	ContinueArgs:       "-c",
	// Bare --resume resumes the most recent session — no picker flag exists.
	ResumeArgs: "--resume",
	Env:        map[string]string{"GROK_DISABLE_AUTOUPDATER": "1"},
	AllowDomains: []string{
		"x.ai",
		"cli-chat-proxy.grok.com",
		"code.grok.com",
		"assets.grok.com",
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
func (c CLI) SessionCmd(shell, cont, resume bool, args []string) []string {
	if shell {
		return nil
	}

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
