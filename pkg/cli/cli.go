// Package cli abstracts the coding CLIs ccbox can install and launch
//
// NOTE: Multiple timeframes and actions to separate in chronological order:
//  1. on Load(), called from cmd.rootCmd.PersistentPreRunE() - CLI.yaml are loaded,
//     which cannot have errors to configure `ccbox` itself. They are from:
//     a. embed, where we panic() instead, due compile-time embed constant
//     b. clitmpl.UserDir(), where we log.Warn() and skip instead. Validations on the CLI.yaml and config/
//     dir are done, _effectively guaranteeing_ valid CLI structs and workable config/ dir onwards
//  2. on cmd.rootCmd.PersistentPreRunE():
//     a. clitmpl.UserSeedFS() seeds the clitmpl.UserDir()
//     b. projectcfg.Load() validates projectcfg.Config, which given 1b., _effectively guarantees_ any
//     cliName passed down will match a CLI in All()
//  3. MustFor() is also called in multiple places. 2b. should be the earliest guard for cliName passed down
//  4. clitmpl.UserCLIConfigSeedFS() for seeding the config to configure the running containerized CLI
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"
	"gopkg.in/yaml.v3"

	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/pkg/cli/clitmpl"
	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/maputil"
	"github.com/s12chung/ccbox/pkg/util/must"
)

// Load loads the cli set
func Load() { all = mustLoadAll() }

// all is every known CLI by name, loaded in the init() at the bottom of this file.
var all map[string]CLI

// All lists every supported CLI, sorted by name.
func All() []CLI {
	clis := make([]CLI, 0, len(all))
	for _, name := range Names() {
		clis = append(clis, all[name])
	}
	return clis
}

// Names lists every supported CLI's name, sorted by name.
func Names() []string { return slices.Sorted(maps.Keys(all)) }

// UserConfigDir is the CLI's user config dir: ~/.ccbox/<cli_name>. Tests point HOME at a temp tree.
func UserConfigDir(cliName string) string {
	return filepath.Join(userdir.Dir(), MustFor(cliName).Name)
}

// mustLoadAll parses the embedded clis, then merges user-defined ones over
// them: a user cli takes priority over an embedded cli of the same name,
// replacing it.
func mustLoadAll() map[string]CLI {
	// unreachable: the source is a compile-time embed constant
	embedClis := must.Get(wrapTreeLoad(clitmpl.Load(clitmpl.EmbedTree(), parseCLI)))
	// unreachable: UserTree() should never return an error
	userClis := must.Get(wrapTreeLoad(clitmpl.Load(clitmpl.UserTree(), parseCLI)))

	all := make(map[string]CLI, len(embedClis)+len(userClis))
	for _, c := range slices.Concat(embedClis, userClis) {
		all[c.Name] = c
	}
	return all
}

// parseCLI decodes one CLI.yaml body into its CLI, named name, with isUserDefined
// set from its tree — firm-validated to fail at startup, not on first use.
func parseCLI(name string, body []byte, isUserDefined bool) (CLI, error) {
	var c CLI
	dec := yaml.NewDecoder(bytes.NewReader(body))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return CLI{}, fmt.Errorf("cli: parse %s: %w", name, err)
	}

	c.Name = name
	c.isUserDefined = isUserDefined
	if errMap := firm.ValidateAny(c); errMap != nil {
		return CLI{}, fmt.Errorf("cli: parse %s: %w", name, errMap)
	}
	return c, nil
}

// wrapTreeLoad runs a clis tree load, printing its warnings
func wrapTreeLoad(clis []CLI, warns []error, err error) ([]CLI, error) {
	for _, warn := range warns {
		log.Warnf("%s, skipping", warn)
	}
	return clis, err
}

// LoadUserClis loads just the user clis tree, returning its load warnings
func LoadUserClis() ([]CLI, []error, error) {
	return clitmpl.Load(clitmpl.UserTree(), parseCLI)
}

// CLI holds everything ccbox does differently per coding CLI.
type CLI struct {
	// PkgInfo is the CLI's install source: its name plus exactly one of Npm or VersionURL;
	// Name is always overridden by parseCLI from the cli dir's name
	pkginfo.PkgInfo `yaml:",inline"`

	// isUserDefined is true if the CLI is user-defined, not from the embedded tree
	isUserDefined bool

	// ConfigHomeMount is the CLI's native default config dir in-container within the $HOME, so the
	// mounted config is found with no override; ConfigDirEnvKey points the CLI's env var at it.
	ConfigHomeMount string `yaml:"config_home_mount"`

	// ConfigDirEnvKey is the env var naming the mounted config path for this CLI
	// (e.g. CLAUDE_CONFIG_DIR), set per run by pkg/docker.
	ConfigDirEnvKey *string `yaml:"config_dir_env_key"`

	// Env is fixed container env the CLI requires (updater/traffic toggles), merged into
	// every run of this CLI.
	Env map[string]string `yaml:"env"`

	// Cmd is the launch argv prefix; ContinueArgs/ResumeArgs extend it for the
	// run flags (-c/--resume) in each CLI's own session syntax.
	Cmd          string `yaml:"cmd"`
	ContinueArgs string `yaml:"continue_args"`
	ResumeArgs   string `yaml:"resume_args"`

	// AllowDomains are the egress wall domains this CLI talks to.
	AllowDomains []string `yaml:"allow_domains"`

	// DataBinds binds host per-CLI data (~/.ccbox/data/<cli>/<path-slug>) into the container.
	// Key is a $HOME-relative path (file or dir); value is seed content for files, nil for dirs.
	DataBinds map[string]*string `yaml:"data_binds"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[CLI]().
		Validates(firm.RuleMap{
			"PkgInfo": {firm.Backed()},

			"Cmd":          {rule.Present{}},
			"ContinueArgs": {rule.Present{}},
			"ResumeArgs":   {rule.Present{}},

			"ConfigHomeMount": {firmrule.HomePath},
			"ConfigDirEnvKey": {firmrule.EnvVar},
			"Env": {
				firm.Keys[map[string]string](firmrule.EnvVar),
				firm.Values[map[string]string](rule.Present{}),
			},
			"AllowDomains": {firm.Elems[[]string](firmrule.Domain)},

			// MaskDir over HomePath: keys are $HOME-relative paths, and MaskDir's
			// no-leading-".." also bars path.Join escapes out of the home dir
			"DataBinds": {firm.Keys[map[string]*string](firmrule.MaskDir)},
		}))
}

// IsUserDefined is true if the CLI is user-defined, not from the embedded tree
func (c CLI) IsUserDefined() bool { return c.isUserDefined }

// PkgInfoJSON renders c's install source as the CLI_PKGINFO JSON for the container env.
func (c CLI) PkgInfoJSON() (string, error) {
	body, err := json.Marshal(c.PkgInfo)
	if err != nil {
		return "", fmt.Errorf("cli: %w", err)
	}
	return string(body), nil
}

// For looks up the CLI by name. ok is false for an unknown name.
func For(name string) (CLI, bool) { return maputil.Get(all, name) }

// MustFor is For for names already validated (projectcfg.Load rejects unknown
// cli values); it panics on an unknown name.
func MustFor(name string) CLI {
	c, ok := For(name)
	if !ok {
		panic(fmt.Sprintf("cli: unknown cli %q", name))
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
