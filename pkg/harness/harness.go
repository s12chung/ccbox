// Package harness abstracts the coding CLIs ccbox can install and launch
//
// NOTE: Multiple timeframes and actions to separate in chronological order:
//  1. on init() - CLI.yaml are loaded, which cannot have errors to configure `ccbox` itself. They are from:
//     a. embed, where we panic() instead, due compile-time embed constant
//     b. UserCLIsDir(), where we log.Warn() and skip instead. Validations on the CLI.yaml and config/
//     dir are done, _effectively guaranteeing_ valid CLI structs and workable config/ dir onwards
//  2. on cmd.rootCmd.PersistentPreRunE():
//     a. SeedUserClisFS() seeds the UserCLIsDir()
//     b. projectcfg.Load() validates projectcfg.Config, which given 1b., _effectively guarantees_ any
//     cliName passed down will match a CLI in All()
//  3. MustFor() is also called in multiple places. 2b. should be the earliest guard for cliName passed down
//  4. SeedCLIFS() for seeding the config to configure the running containerized CLI
package harness

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"

	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/fsutil"
	"github.com/s12chung/ccbox/pkg/util/maputil"
)

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

// clis tree layout: <root>/clis/<name>/CLI.yaml plus <root>/clis/<name>/config/
const (
	clisDir       = "clis"
	clisYAMLGlob  = "clis/*/CLI.yaml"
	seedConfigDir = "config"
)

// cliDir is a cli's dir in a clis tree: clis/<name>
func cliDir(name string) string { return path.Join(clisDir, name) }

// cliConfigPath is a cli's seed config tree in a clis tree: clis/<name>/config
func cliConfigPath(name string) string { return path.Join(cliDir(name), seedConfigDir) }

// UserCLIsDir is the host dir of user-defined clis: ~/.ccbox/config/clis.
func UserCLIsDir() string { return filepath.Join(userConfigDir, clisDir) }

// userCLIsFS returns the user clis tree at userConfigDir
// DOES NOT detect whether the directory exists, this should be detected on init()--see package NOTE
func userCLIsFS() fs.FS { return os.DirFS(userConfigDir) }

var (
	//go:embed clis
	embedCLIFS embed.FS
	// userConfigDir is the user config dir: ~/.ccbox/config. Tests point it at a temp tree.
	userConfigDir = userdir.ConfigDir()
)

// SeedCLIFS returns cli's seed fs: the CLI's own config tree, from embed or
// user-clis from UserCLIsDir().
func SeedCLIFS(cliName string) fs.FS {
	cli := MustFor(cliName)
	fsys := fs.FS(embedCLIFS)
	if cli.fromUserDir {
		vfs := fsutil.MustNewFS(userCLIsFS())
		// vfs.MkdirAll ensures cliConfigPath exists for the caller, no-op if the dir exists
		if err := vfs.MkdirAll(cliConfigPath(cliName)); err != nil { // ensures MustSub() doesn't panic
			panic(err) // unreachable: loadFsYAML skips user clis whose clis/<cliName>/config is a file
		}
		fsys = vfs
	}
	// any cliName passed down will match a CLI in All() - see package NOTE
	return fsutil.MustSub(fsys, cliConfigPath(cliName))
}

// mustLoadAll parses the embedded clis, then merges user-defined ones over
// them: a user cli takes priority over an embedded cli of the same name,
// replacing it.
func mustLoadAll() map[string]CLI {
	embed, user := mustLoadEmbedCLIs(), mustLoadUserCLIs()
	all := make(map[string]CLI, len(embed)+len(user))
	for _, c := range slices.Concat(embed, user) {
		if _, ok := all[c.Name]; ok {
			log.Infof("user cli %q overrides the embedded cli", c.Name)
		}
		all[c.Name] = c
	}
	return all
}

// mustLoadEmbedCLIs parses each embedded clis/<cli>/CLI.yaml into a CLI named <cli>,
// ordered by name. It panics on any load error.
func mustLoadEmbedCLIs() []CLI {
	clis, err := embedTree().load()
	if err != nil {
		panic(err) // unreachable: the source is a compile-time embed constant
	}
	return clis
}

// mustLoadUserCLIs parses each user-defined <cli>/CLI.yaml in the user clis tree,
// skipping broken ones with a warning.
func mustLoadUserCLIs() []CLI {
	clis, err := userTree().load()
	if err != nil {
		panic(err) // unreachable: userTree().load() should never return an error
	}
	return clis
}

// CLI holds everything ccbox does differently per coding CLI.
type CLI struct {
	// PkgInfo is the CLI's install source: its name plus exactly one of Npm or VersionURL;
	// Name is always overridden by defaulted() from the CLI's directory name
	pkginfo.PkgInfo `yaml:",inline"`

	// fromUserDir is true if it's from the user dir
	fromUserDir bool

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

// PkgInfoJSON renders c's install source as the CLI_PKGINFO JSON for the container env.
func (c CLI) PkgInfoJSON() (string, error) {
	body, err := json.Marshal(c.PkgInfo)
	if err != nil {
		return "", fmt.Errorf("harness: %w", err)
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
		panic(fmt.Sprintf("harness: unknown cli %q", name))
	}
	return c
}

//go:embed user-clis
var userCLIFSSeed embed.FS

// SeedUserClisFS returns the embedded tree laid onto a fresh user clis dir.
func SeedUserClisFS() fs.FS { return fsutil.MustSub(userCLIFSSeed, "user-clis") }

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

// init() loads after the firm registration following the CLI struct, which parse() validates against.
func init() { all = mustLoadAll() }
