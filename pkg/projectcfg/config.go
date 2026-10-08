package projectcfg

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"text/template"

	"github.com/gobwas/glob"
	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"

	"github.com/s12chung/ccbox/pkg/cli"
	"github.com/s12chung/ccbox/pkg/guiapp"
	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/kit/git"
	"github.com/s12chung/ccbox/pkg/kit/globkit"
	"github.com/s12chung/ccbox/pkg/kit/yamlutil"
	"github.com/s12chung/ccbox/pkg/projectcfg/expand"
	"github.com/s12chung/ccbox/pkg/provider"
	"github.com/s12chung/ccbox/pkg/runtime"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/maputil"
	"github.com/s12chung/ccbox/pkg/util/mergeempty"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

// ContainerHome is the container user's home: read_only_binds' ~/ mounts sit under it
const ContainerHome = "/home/ccbox"

// The read_only_binds' gitconfig special entry: the key carries the host's XDG git config
// dir, and the value mounts it at its default container path
const (
	// GitConfigKey is read_only_binds' special key: the host's XDG git config dir
	GitConfigKey = "gitconfig"
	// GitConfigMount is the gitconfig entry's container mount (git's default XDG path)
	GitConfigMount = ContainerHome + "/.config/git"
)

// Config is the parsed .ccbox.yaml.
type Config struct {
	CLIName       *string           `yaml:"cli"`                // coding CLI to install + launch
	NoProxy       bool              `yaml:"no_proxy,omitempty"` // skip the egress wall: direct network access
	VNC           *VNC              `yaml:"vnc"`                // the desktop's session: its GUI app
	TmpfsMasks    []string          `yaml:"tmpfs_masks"`        // project-relative dirs to mask with a writable tmpfs
	VolumeMasks   []string          `yaml:"volume_masks"`       // project-relative dirs to mask with a persistent per-project volume
	ReadOnlyGlobs []string          `yaml:"read_only_globs"`    // project-relative globs to re-mount read-only
	ReadOnlyBinds map[string]string `yaml:"read_only_binds"`    // host dir → container mount dir, bound read-only
	Env           map[string]string `yaml:"env"`                // extra env vars set in the container
	Allowlist     []string          `yaml:"allowlist"`          // egress wall domains

	// projectDir anchors the masks' present-filters
	projectDir string
	// serveVNC is the resolved run mode — the --vnc flag or the vnc section's enabled:
	// the GUI app's domains ride SetHarnessAlias's expansion only on it
	serveVNC bool
	// cache the DefaultsAlias expansions of raw lists and the read_only_binds' path resolutions
	expandedTmpfsMasks    []string
	expandedVolumeMasks   []string
	expandedReadOnlyGlobs []string
	expandedReadOnlyBinds map[string]string
	expandedAllowlist     []string
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[Config]().
		NotNil("CLIName").
		Validates(firm.RuleMap{
			"CLIName": {rule.OneOf[string]{ValuesFunc: cli.Names}},
			"VNC":     {firm.Backed()},

			"TmpfsMasks":    {firm.Elems[[]string](rule.ShowValue(firmrule.MaskDir))},
			"VolumeMasks":   {firm.Elems[[]string](rule.ShowValue(firmrule.MaskDir))},
			"ReadOnlyGlobs": {firm.Elems[[]string](rule.ShowValue(firmrule.MaskGlob))},

			// host dir → container mount dir, bound read-only
			"ReadOnlyBinds": {firm.KeyValues[map[string]string](firmrule.Bind{
				Specials: []string{GitConfigKey},
			})},

			"Env": {
				firm.Keys[map[string]string](firmrule.EnvVar),
				firm.Values[map[string]string](rule.Present{}),
			},
			"Allowlist": {firm.Elems[[]string](rule.ShowValue(firmrule.DomainOrAlias(allowlistAliases)))},
		}))
}

// VNC is the desktop's session: the GUI app it installs and launches.
type VNC struct {
	// Enabled always runs with VNC served — the --vnc flag's config form
	Enabled bool `yaml:"enabled,omitempty"`
	// GUIAppName is the GUI app's name (guiapp.Names()); empty means no GUI app
	GUIAppName string `yaml:"gui_app,omitempty"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[VNC]().
		Validates(firm.RuleMap{
			"GUIAppName": {rule.OneOf[string]{
				Values: append(guiapp.Names(), ""), // empty name means no GUI app to run
			}},
		}))
}

// ProjectDir is the project dir the config was loaded from
func (c *Config) ProjectDir() string { return c.projectDir }

// ServeVNC is the resolved run mode: the --vnc flag or the vnc section's enabled
func (c *Config) ServeVNC() bool { return c.serveVNC }

// CLI resolves the config's cli name to its cli.CLI; MustFor is
// infallible for a loaded config
func (c *Config) CLI() cli.CLI { return cli.MustFor(*c.CLIName) }

// merge layers other onto c, resetting the expanded caches
func (c *Config) merge(other Config) {
	mergeempty.Merge(c, other) // unexported fields are not merged
	c.expandedTmpfsMasks = nil
	c.expandedVolumeMasks = nil
	c.expandedReadOnlyGlobs = nil
	c.expandedReadOnlyBinds = nil
	c.expandedAllowlist = nil
}

// TmpfsMasksExpanded expands the DefaultsAlias alias in TmpfsMasks to defaults
func (c *Config) TmpfsMasksExpanded() []string {
	if c.expandedTmpfsMasks == nil {
		c.expandedTmpfsMasks = expandDefaultAlias(c.TmpfsMasks, tmpfsDefaults)
	}
	return c.expandedTmpfsMasks
}

// TmpfsMasksPresent filters TmpfsMasksExpanded() to the paths present as dirs in the project
func (c *Config) TmpfsMasksPresent() []string {
	return ioutil.DirsPresent(c.projectDir, c.TmpfsMasksExpanded())
}

// TmpfsMasksAbsent filters TmpfsMasksExpanded() to the paths NOT present as dirs in the project
func (c *Config) TmpfsMasksAbsent() []string {
	return absentDirs(c.projectDir, c.TmpfsMasksExpanded())
}

// VolumeMasksExpanded expands the DefaultsAlias alias in VolumeMasks to defaults
func (c *Config) VolumeMasksExpanded() []string {
	if c.expandedVolumeMasks == nil {
		c.expandedVolumeMasks = expandDefaultAlias(c.VolumeMasks, volumeDefaults)
	}
	return c.expandedVolumeMasks
}

// VolumeMasksPresent filters VolumeMasksExpanded() to the paths present as dirs in the project
func (c *Config) VolumeMasksPresent() []string {
	return ioutil.DirsPresent(c.projectDir, c.VolumeMasksExpanded())
}

// VolumeMasksAbsent filters VolumeMasksExpanded() to the paths NOT present as dirs in the project
func (c *Config) VolumeMasksAbsent() []string {
	return absentDirs(c.projectDir, c.VolumeMasksExpanded())
}

// ReadOnlyGlobsExpanded expands the DefaultsAlias alias in ReadOnlyGlobs to defaults
func (c *Config) ReadOnlyGlobsExpanded() []string {
	if c.expandedReadOnlyGlobs == nil {
		c.expandedReadOnlyGlobs = expandDefaultAlias(c.ReadOnlyGlobs, readOnlyDefaults)
	}
	return c.expandedReadOnlyGlobs
}

func (c *Config) readOnlyGlobsMatchers() []glob.Glob {
	globStrings := c.ReadOnlyGlobsExpanded()
	globs := make([]glob.Glob, 0, len(globStrings))
	for _, g := range globStrings {
		globs = append(globs, globkit.DoubleStarRooted(g))
	}
	return globs
}

// ReadOnlyPathsPresent matches ReadOnlyGlobsExpanded() against the project to actual paths,
// minus masked dirs and covered matches — a kept match covers the matches under it
func (c *Config) ReadOnlyPathsPresent() []string {
	covered := map[string]bool{}
	for _, mask := range slices.Concat(c.TmpfsMasksExpanded(), c.VolumeMasksExpanded()) {
		covered[mask] = true
	}
	var paths []string
	for _, match := range globkit.WalkMatches(c.projectDir, c.readOnlyGlobsMatchers()...) {
		dir := match
		for dir != "." && !covered[dir] {
			dir = filepath.Dir(dir)
		}
		if dir != "." {
			continue // a mask or a kept match covers it
		}
		covered[match] = true
		paths = append(paths, match)
	}
	slices.Sort(paths)
	return paths
}

// ManyReadOnlyMatches reports whether ReadOnlyPathsPresent() matches globkit.MatchLimit
// paths or more — a strayed `ccbox config` at home or / globs into everywhere. The walk
// quits at the limit.
func (c *Config) ManyReadOnlyMatches() bool {
	return globkit.ManyMatches(c.projectDir, c.readOnlyGlobsMatchers()...)
}

// ReadOnlyBindsExpanded resolves the raw binds to host dir → container mount: the gitconfig
// entry to the host's XDG git dir at GitConfigMount, the rest's ~/ paths to their own side's
// home.
func (c *Config) ReadOnlyBindsExpanded() map[string]string {
	if c.expandedReadOnlyBinds == nil {
		c.expandedReadOnlyBinds = expand.Map(c.ReadOnlyBinds,
			map[string]expand.Expansion{
				// gitconfig: the host's XDG git dir at its default mount
				GitConfigKey: {
					Key:   git.XDGConfigDir(),
					Value: func(string) string { return GitConfigMount },
				},
			},
			func(key, mount string) (string, string) { // ~/ resolves to their own side's home
				return ioutil.ExpandHome(key, userdir.MustHome()), ioutil.ExpandHome(mount, ContainerHome)
			},
		)
	}
	return c.expandedReadOnlyBinds
}

// ReadOnlyBindsPresent filters ReadOnlyBindsExpanded() to the host dirs present on disk
func (c *Config) ReadOnlyBindsPresent() map[string]string {
	return presentKeys(c.ReadOnlyBindsExpanded())
}

// ReadOnlyBindsAbsent filters ReadOnlyBindsExpanded() to the host dirs NOT present on disk
func (c *Config) ReadOnlyBindsAbsent() map[string]string {
	return absentKeys(c.ReadOnlyBindsExpanded(), c.ReadOnlyBindsPresent())
}

// AllowlistExpanded expands Allowlist's aliases in place
func (c *Config) AllowlistExpanded() []string {
	if c.expandedAllowlist == nil {
		c.expandedAllowlist = c.expandAllowlist()
	}
	return c.expandedAllowlist
}

// SetHarnessDomains is what SetHarnessAlias expands to on the config: the selected
// CLI's egress domains, its provider aliases expanded, plus the GUI app's download
// domains on a vnc load — only a desktop run downloads the app.
func (c *Config) SetHarnessDomains() []string {
	var domains []string
	for _, d := range c.CLI().AllowDomains {
		domains = append(domains, expandDomain(d)...)
	}
	if c.serveVNC && c.VNC != nil {
		domains = append(domains, guiapp.AllowDomains(c.VNC.GUIAppName)...)
	}
	return domains
}

// NoProxyFor resolves the run's skip-the-proxy — a vnc run always skips it
func (c *Config) NoProxyFor(serveVNC, flag bool) bool {
	return serveVNC || flag || c.NoProxy
}

// expandAllowlist runs the alias→domains expansion over the raw entries, keeping order
// and dropping repeats. Cycle-free: validation bars DefaultsAlias/SetHarnessAlias from
// a CLI.yaml, so SetHarnessDomains never re-enters the aliases.
func (c *Config) expandAllowlist() []string {
	expansions := map[string][]string{
		DefaultsAlias:   AllowlistDefaults(),
		SetHarnessAlias: c.SetHarnessDomains(),
	}
	for _, alias := range slices.Concat(provider.Aliases(), runtime.Aliases()) {
		expansions[alias] = expandDomain(alias)
	}
	return expand.Slice(c.Allowlist, expansions)
}

// allowlistAliases lists every allowlist alias: the providers' and runtimes' plus the
// expansion aliases
func allowlistAliases() []string {
	return slices.Concat(provider.Aliases(), runtime.Aliases(), []string{DefaultsAlias, SetHarnessAlias})
}

// expandDefaultAlias replaces each DefaultsAlias with defaults, preserving entry order and
// dropping repeat entries.
func expandDefaultAlias(list, defaults []string) []string {
	return expand.Slice(list, map[string][]string{DefaultsAlias: defaults})
}

// expandDomain resolves one entry: a provider or runtime alias to its table domains,
// else the raw domain
func expandDomain(entry string) []string {
	for _, resolve := range []func(string) ([]string, bool){provider.DomainsFor, runtime.DomainsFor} {
		if domains, ok := resolve(entry); ok {
			return domains
		}
	}
	return []string{entry}
}

// MarshalYAML renders the effective config: masks resolved to the project's present
// dirs, read_only_globs to their matched paths, read_only_binds to the host's present dirs,
// list aliases expanded — what `ccbox config` prints.
func (c *Config) MarshalYAML() (any, error) {
	type noMarshalYamlDefined Config
	return noMarshalYamlDefined{
		CLIName:       c.CLIName,
		NoProxy:       c.NoProxy,
		VNC:           c.VNC,
		TmpfsMasks:    c.TmpfsMasksPresent(),
		VolumeMasks:   c.VolumeMasksPresent(),
		ReadOnlyGlobs: c.ReadOnlyPathsPresent(),
		ReadOnlyBinds: c.ReadOnlyBindsPresent(),
		Env:           c.Env,
		Allowlist:     c.AllowlistExpanded(),
	}, nil
}

// InfoJSON renders the section as the VNCConfigEnvVar JSON: the GUI app the run
// installs and launches
func (v *VNC) InfoJSON() (string, error) {
	var info pkginfo.VNCInfo
	if v.GUIAppName != "" {
		app, err := guiapp.For(v.GUIAppName)
		if err != nil {
			return "", err
		}
		info.GUIApp = &app.GUIPkgInfo
	}
	body, err := json.Marshal(info)
	if err != nil {
		return "", fmt.Errorf("vnc config: %w", err)
	}
	return string(body), nil
}

var (
	// configTmplSrc is the ccbox.yaml template
	//
	//go:embed ccbox.yaml.tmpl
	configTmplSrc string
	tmpl          = template.Must(template.New("ccbox.yaml").Funcs(template.FuncMap{
		"yaml": yamlutil.Value,
	}).Parse(configTmplSrc))
)

// configTmpl wraps Config with per-entry comments for the fields that have them; the template
// passes nil for the rest, whose yaml calls render bare
type configTmpl struct {
	Config

	TmpfsMasksComments    []string
	VolumeMasksComments   []string
	ReadOnlyGlobsComments []string
	AllowlistComments     []string
}

const defaultsExpansionComment = "`ccbox config defaults` for expansions"

func (c *Config) renderTmpl() (string, error) {
	var b bytes.Buffer
	data := configTmpl{
		Config:                *c,
		TmpfsMasksComments:    yamlutil.CommentFirstEntry(c.TmpfsMasks, defaultsExpansionComment),
		VolumeMasksComments:   yamlutil.CommentFirstEntry(c.VolumeMasks, defaultsExpansionComment),
		ReadOnlyGlobsComments: yamlutil.CommentFirstEntry(c.ReadOnlyGlobs, defaultsExpansionComment),
		AllowlistComments: yamlutil.EntryToComments(c.Allowlist, map[string]string{
			DefaultsAlias:            "shared tooling domains",
			runtime.AllRuntimesAlias: "every runtime's domains (node, python, ruby, go)",
			SetHarnessAlias:          "the selected CLI's — and on --vnc, the GUI app's",
		}),
	}
	if err := tmpl.Execute(&b, data); err != nil {
		return "", fmt.Errorf("render Config template: %w", err)
	}
	return b.String(), nil
}

func absentDirs(projectDir string, dirs []string) []string {
	present := ioutil.DirsPresent(projectDir, dirs)
	var out []string
	for _, d := range dirs {
		if !slices.Contains(present, d) {
			out = append(out, d)
		}
	}
	return out
}

// presentKeys filters binds to the keys present as dirs on disk
func presentKeys(binds map[string]string) map[string]string {
	present := make(map[string]string, len(binds))
	for key, value := range binds {
		if info, err := os.Stat(key); err == nil && info.IsDir() { // #nosec G703 -- key is the user's own dir
			present[key] = value
		}
	}
	return maputil.NilIfEmpty(present)
}

// absentKeys filters binds to the keys missing from present
func absentKeys(binds, present map[string]string) map[string]string {
	absent := make(map[string]string, len(binds))
	for key, value := range binds {
		if _, ok := present[key]; !ok {
			absent[key] = value
		}
	}
	return maputil.NilIfEmpty(absent)
}
