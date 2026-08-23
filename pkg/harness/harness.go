// Package harness abstracts the coding CLIs ccbox can install and launch
package harness

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/s12chung/ccbox/pkg/kit/pkger"
)

var all []CLI

func init() { all = MustLoad() }

// All lists every supported CLI, in stable order.
func All() []CLI { return slices.Clone(all) }

const (
	// SeedConfigDir is the per-CLI subdirectory holding the CLI's own seed tree.
	SeedConfigDir = "config"
	// SharedSeedPath is the seed subtree shared across every CLI.
	SharedSeedPath = "shared"
	// AgentsFileName is the shared user AGENTS.md
	AgentsFileName = "AGENTS.user.md"
)

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

// MustLoad parses each embedded clis/<cli>/CLI.yaml into a CLI named <cli>,
// ordered by name. It panics on any parse error.
func MustLoad() []CLI {
	paths, err := fs.Glob(clisFS, "clis/*/CLI.yaml")
	if err != nil {
		panic(err) // unreachable: the pattern above is a valid glob
	}

	clis := make([]CLI, 0, len(paths))
	for _, p := range paths {
		body, err := fs.ReadFile(clisFS, p)
		if err != nil {
			panic(err) // unreachable: the path comes from Glob above
		}
		clis = append(clis, mustParse(p, body))
	}
	if len(clis) == 0 {
		panic("harness: no clis/*/CLI.yaml found")
	}
	return clis
}

// mustParse decodes one CLI.yaml into its CLI, named after its directory.
func mustParse(p string, body []byte) CLI {
	var c CLI
	dec := yaml.NewDecoder(bytes.NewReader(body))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		panic(fmt.Sprintf("harness: parse %s: %v", p, err))
	}

	c.Pkger() // fail at startup, not on first use
	c.Name = path.Base(path.Dir(p))
	return parseDefaulted(c)
}

// parseDefaulted
func parseDefaulted(c CLI) CLI {
	if c.SeedAgentsFilename == "" {
		c.SeedAgentsFilename = "AGENTS.md"
	}
	return c
}

// CLI holds everything ccbox does differently per coding CLI.
type CLI struct {
	Name string `yaml:"-"`

	// Npm installs from the npm registry. Exactly one of Npm or VersionURL is set;
	// Pkger returns whichever it is.
	Npm        *pkger.Npm        `yaml:"npm"`
	VersionURL *pkger.VersionURL `yaml:"versionurl"`

	// ConfigHomeMount is the CLI's native default config dir in-container within the $HOME, so the
	// mounted config is found with no override; ConfigDirEnvKey points the CLI's env var at it.
	ConfigHomeMount string `yaml:"config_home_mount"`

	// ConfigDirEnvKey is the env var naming the mounted config path for this CLI
	// (e.g. CLAUDE_CONFIG_DIR), set per run by pkg/docker; "" = none.
	ConfigDirEnvKey string `yaml:"config_dir_env_key"`

	// Env is fixed container env the CLI requires (updater/traffic toggles), merged into
	// every run of this CLI.
	Env map[string]string `yaml:"env"`

	// SeedAgentsFilename is the destination name of the shared/ AGENTS.md
	//
	// Empty defaults to AGENTS.md at parse.
	SeedAgentsFilename string `yaml:"seed_agents_filename"`

	// Cmd is the launch argv prefix; ContinueArgs/ResumeArgs extend it for the
	// run flags (-c/--resume) in each CLI's own session syntax.
	Cmd          string `yaml:"cmd"`
	ContinueArgs string `yaml:"continue_args"`
	ResumeArgs   string `yaml:"resume_args"`

	// AllowDomains are the egress wall domains this CLI talks to.
	AllowDomains []string `yaml:"allow_domains"`
}

// Pkger returns the install source this CLI declares: Npm or VersionURL.
func (c CLI) Pkger() pkger.Pkger {
	switch {
	case c.Npm != nil && c.VersionURL == nil:
		return *c.Npm
	case c.VersionURL != nil && c.Npm == nil:
		return *c.VersionURL
	default:
		panic("pkger: want exactly one of npm, versionurl") // unreachable: MustLoad rejects such YAML
	}
}

// For looks up the CLI by name. ok is false for an unknown name.
func For(name string) (CLI, bool) {
	for _, c := range All() {
		if c.Name == name {
			return c, true
		}
	}
	return CLI{}, false
}

// MustFor is For for names already validated (projectcfg.Load rejects unknown
// cli values); it panics on an unknown name.
func MustFor(name string) CLI {
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
