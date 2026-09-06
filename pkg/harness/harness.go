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
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"
	"gopkg.in/yaml.v3"

	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/kit/pkger"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/fsutil"
	"github.com/s12chung/ccbox/pkg/util/log"
)

// all is every known CLI, loaded in the init() at the bottom of this file.
var all []CLI

// All lists every supported CLI, in stable order.
func All() []CLI { return slices.Clone(all) }

// Names lists every supported CLI's name, in All's order.
func Names() []string {
	all := All()
	names := make([]string, 0, len(all))
	for _, c := range all {
		names = append(names, c.Name)
	}
	return names
}

const (
	// SeedConfigDir is the per-CLI subdirectory holding the CLI's own seed tree.
	SeedConfigDir = "config"
	// AgentsFileName is the shared user AGENTS.md
	AgentsFileName = "AGENTS.user.md"
)

var (
	//go:embed clis
	embedCLIFS embed.FS
	//go:embed shared-clis
	sharedCLIFS embed.FS
	//go:embed user-clis
	userCLIFS embed.FS
)

// SeedCLIFS returns cli's seed fs--shared-clis/ with renames from embed + (clis from embed or user-clis from UserCLIsDir())
// merged into one tree. The embed cli merges last, so it owns colliding paths when seeded.
//
// This fs is used for seeding the running containerized CLI
func SeedCLIFS(cliName string) *fsutil.FS {
	cli := MustFor(cliName)
	sharedFS := fsutil.MustNewFS(fsutil.MustSub(sharedCLIFS, path.Join("shared-clis", SeedConfigDir)))
	if err := sharedFS.Rename(AgentsFileName, cli.SeedAgentsFilename); err != nil {
		panic(err) // unreachable: the source is a compile-time embed constant
	}

	cliConfigPath := path.Join("clis", cliName, SeedConfigDir)
	fsys := fs.FS(embedCLIFS)
	if cli.fromUserDir {
		vfs := fsutil.MustNewFS(userCLIsFS())
		if err := vfs.MkdirAll(cliConfigPath); err != nil { // ensure MustSub() doesn't panic, no-op if dir path already exists
			panic(err) // unreachable: loadFsYAML skips user clis whose clis/<cliName>/config is a file
		}
		fsys = vfs
	}
	// any cliName passed down will match a CLI in All() - see package NOTE
	return sharedFS.MustMerge(fsutil.MustSub(fsys, cliConfigPath))
}

// UserCLIsDir is the host dir of user-defined clis: ~/.ccbox/config/clis.
func UserCLIsDir() string { return filepath.Join(userdir.ConfigDir(), "clis") }

// userConfigDir is the user config dir: ~/.ccbox/config. Tests point it at a temp tree.
var userConfigDir = userdir.ConfigDir()

// userCLIsFS returns the user clis tree at userConfigDir
// DOES NOT detect whether the directory exists, this should be detected on init()--see package NOTE
func userCLIsFS() fs.FS { return os.DirFS(userConfigDir) }

// mustLoadAll parses the embedded clis, then merges user-defined ones beneath
// them, sorted by name: a user cli conflicting with an embedded name is skipped
// with a warning.
func mustLoadAll() []CLI {
	clis := mustLoadEmbedCLIs()
	for _, c := range loadUserCLIs() {
		if slices.ContainsFunc(clis, func(e CLI) bool { return e.Name == c.Name }) {
			log.Warnf("skipping user cli %q: conflicts with an embedded cli", c.Name)
			continue
		}
		clis = append(clis, c)
	}
	slices.SortFunc(clis, func(a, b CLI) int { return strings.Compare(a.Name, b.Name) })
	return clis
}

// mustLoadEmbedCLIs parses each embedded clis/<cli>/CLI.yaml into a CLI named <cli>,
// ordered by name. It panics on any parse error.
func mustLoadEmbedCLIs() []CLI {
	clis, err := loadFsYAML(embedCLIFS, false)
	if err != nil {
		panic(err) // unreachable: the source is a compile-time embed constant
	}
	if len(clis) == 0 {
		panic("harness: no clis/*/CLI.yaml found")
	}
	return clis
}

// loadUserCLIs parses each user-defined <cli>/CLI.yaml in the user clis tree,
// skipping broken ones with a warning.
func loadUserCLIs() []CLI {
	clis, err := loadFsYAML(userCLIsFS(), true)
	if err != nil {
		panic(err) // unreachable: loadFsYAML(_, true) should never return an error
	}
	return clis
}

// loadFsYAML parses each <root>/clis/<cli>/CLI.yaml under fsys into a CLI named
// <cli>, ordered by name. A user tree (fromUserDir) skips broken entries with a
// warning; an embed tree fails instead — its contents are compile-time constants.
func loadFsYAML(fsys fs.FS, fromUserDir bool) ([]CLI, error) {
	paths, err := fs.Glob(fsys, "clis/*/CLI.yaml")
	if err != nil {
		// unreachable: glob never changes
		return nil, err
	}

	clis := make([]CLI, 0, len(paths))
	for _, p := range paths {
		body, err := fs.ReadFile(fsys, p)
		if err != nil {
			if fromUserDir {
				log.Warnf("read error in %q: %s, skipping", cliNameFromPath(p), err)
				continue
			}
			return nil, err
		}
		c, err := parse(p, body)
		if err != nil {
			if fromUserDir {
				log.Warnf("parse error in %q: %s, skipping", cliNameFromPath(p), err)
				continue
			}
			return nil, err
		}
		if err := validateSeedConfigDir(fsys, path.Dir(p), fromUserDir); err != nil {
			if fromUserDir {
				log.Warnf("bad seed config in %q: %s, skipping", cliNameFromPath(p), err)
				continue
			}
			return nil, err
		}
		c.fromUserDir = fromUserDir
		clis = append(clis, c)
	}
	return clis, nil
}

// parse decodes one CLI.yaml into its CLI, named after its directory.
func parse(p string, body []byte) (CLI, error) {
	var c CLI
	dec := yaml.NewDecoder(bytes.NewReader(body))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return CLI{}, fmt.Errorf("harness: parse %s: %w", p, err)
	}

	c = defaulted(p, c)
	if errMap := firm.ValidateAny(c); errMap != nil { // fail at startup, not on first use
		return CLI{}, fmt.Errorf("harness: parse %s: %w", p, errMap)
	}
	return c, nil
}

func cliNameFromPath(p string) string { return path.Base(path.Dir(p)) }

func defaulted(p string, c CLI) CLI {
	c.Name = cliNameFromPath(p)
	if c.SeedAgentsFilename == "" {
		c.SeedAgentsFilename = "AGENTS.md"
	}
	return c
}

// validateSeedConfigDir checks <cliDir>/config: for user trees it may be absent, but must not be a file;
// embed trees also require existence — their contents are compile-time constants.
func validateSeedConfigDir(fsys fs.FS, cliDir string, fromUserDir bool) error {
	info, err := fs.Stat(fsys, path.Join(cliDir, SeedConfigDir))
	switch {
	case fromUserDir && errors.Is(err, fs.ErrNotExist):
		return nil // seeded fresh by SeedCLIFS
	case err != nil:
		return err
	case !info.IsDir():
		return fmt.Errorf("%s: not a directory", path.Join(cliDir, SeedConfigDir))
	}
	return nil
}

// CLI holds everything ccbox does differently per coding CLI.
type CLI struct {
	Name string `yaml:"-"`
	// fromUserDir is true if it's from the user dir
	fromUserDir bool

	// Npm installs from the npm registry. Exactly one of Npm or VersionURL is set;
	// Pkger returns whichever it is.
	Npm        *pkger.Npm        `yaml:"npm"`
	VersionURL *pkger.VersionURL `yaml:"versionurl"`

	// ConfigHomeMount is the CLI's native default config dir in-container within the $HOME, so the
	// mounted config is found with no override; ConfigDirEnvKey points the CLI's env var at it.
	ConfigHomeMount string `yaml:"config_home_mount"`

	// ConfigDirEnvKey is the env var naming the mounted config path for this CLI
	// (e.g. CLAUDE_CONFIG_DIR), set per run by pkg/docker.
	ConfigDirEnvKey *string `yaml:"config_dir_env_key"`

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

func init() {
	firm.MustRegisterType(firm.NewDefinition[CLI]().
		ValidatesSelf(firmrule.DefinedOnce{Fields: []string{"Npm", "VersionURL"}}).
		Validates(firm.RuleMap{
			"Npm":        {firm.Backed()},
			"VersionURL": {firm.Backed()},

			"Cmd":          {rule.Present{}},
			"ContinueArgs": {rule.Present{}},
			"ResumeArgs":   {rule.Present{}},

			"ConfigHomeMount":    {firmrule.HomePath},
			"ConfigDirEnvKey":    {firmrule.EnvVar},
			"SeedAgentsFilename": {firmrule.FileName},
			"Env": {
				firm.Keys[map[string]string](firmrule.EnvVar),
				firm.Values[map[string]string](rule.Present{}),
			},
			"AllowDomains": {firm.Elems[[]string](firmrule.Domain)},
		}))
}

// Pkger returns the install source this CLI declares: Npm or VersionURL.
func (c CLI) Pkger() pkger.Pkger {
	p, err := c.pkger()
	if err != nil {
		panic(err) // unreachable: parse rejects such YAML
	}
	return p
}

// pkger resolves c's install source, failing unless exactly one of Npm/VersionURL is set.
func (c CLI) pkger() (pkger.Pkger, error) {
	switch {
	case c.Npm != nil && c.VersionURL == nil:
		return *c.Npm, nil
	case c.VersionURL != nil && c.Npm == nil:
		return *c.VersionURL, nil
	default:
		return nil, errors.New("pkger: want exactly one of npm, versionurl")
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

// SeedUserClisFS returns the embedded tree laid onto a fresh user clis dir.
func SeedUserClisFS() fs.FS { return fsutil.MustSub(userCLIFS, "user-clis") }

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
