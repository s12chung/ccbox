// Package cfg loads the config file layers — user (ConfigDir), project
// (repo root), projectLocal (git-ignored) — lowest precedence first
package cfg

import (
	"bytes"
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"

	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/kit/yamlutil"
	"github.com/s12chung/ccbox/pkg/models/cli"
	"github.com/s12chung/ccbox/pkg/models/guiapp"
	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/errs"
	"github.com/s12chung/ccbox/pkg/util/mergeempty"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/ptrx"
	"github.com/s12chung/ccbox/pkg/util/seed"
)

const (
	// userConfigFileName is the user-level config's name under ConfigDir (no dot)
	userConfigFileName = "ccbox.yaml"
	// projectConfigFileName is the project-level config's name at the project repo root
	projectConfigFileName = ".ccbox.yaml"
	// projectLocalConfigFileName is the project-local, git-ignored override
	projectLocalConfigFileName = ".ccbox.local.yaml"
	// DefaultsAlias listed in allowlist, expands in place to the shared tooling defaults
	DefaultsAlias = "ccbox-defaults"
	// SetHarnessAlias listed in allowlist, expands in place to the selected CLI's egress
	// domains — plus the GUI app's on a vnc load
	SetHarnessAlias = "ccbox-set-harness"
)

// UserConfigFile is the user-level config's path
func UserConfigFile() string { return filepath.Join(userdir.ConfigDir(), userConfigFileName) }

// layerPaths lists the config file paths in load order: user, project, local
func layerPaths(projectDir string) []string {
	return []string{UserConfigFile(), filepath.Join(projectDir, projectConfigFileName), filepath.Join(projectDir, projectLocalConfigFileName)}
}

// LoadedPaths returns the layer config paths split into loaded and not loaded, each in load order
func LoadedPaths(projectDir string) ([]string, []string) {
	var loaded, notLoaded []string
	for _, path := range layerPaths(projectDir) {
		if osutil.Present(path) {
			loaded = append(loaded, path)
		} else {
			notLoaded = append(notLoaded, path)
		}
	}
	return loaded, notLoaded
}

//go:generate go run ../../tools/toinnermodel-gen -dir .

// config is the yaml-facing .ccbox.yaml read
//
//toinnermodel:gen
type config struct {
	CLIName        *string            `yaml:"cli"`             // coding CLI to install + launch
	DefaultVariant *defaultVariant    `yaml:"default_variant"` // the default variant ask's section — the devbox with no run flags
	VNC            *vnc               `yaml:"vnc"`             // the desktop's session: its GUI app
	TmpfsMasks     *[]string          `yaml:"tmpfs_masks"`     // project-relative dirs to mask with a writable tmpfs
	VolumeMasks    *[]string          `yaml:"volume_masks"`    // project-relative dirs to mask with a persistent per-project volume
	ReadOnlyGlobs  *[]string          `yaml:"read_only_globs"` // project-relative globs to re-mount read-only
	ReadOnlyBinds  *map[string]string `yaml:"read_only_binds"` // host dir → container mount dir, bound read-only
	ForwardEnv     *[]string          `yaml:"forward_env"`     // host env vars forwarded into the container
	Proxy          *proxy             `yaml:"proxy"`           // the egress wall's section
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[config]().
		NotNil("CLIName").
		Validates(firm.RuleMap{
			"CLIName":        {rule.OneOf[string]{ValuesFunc: cli.Names}},
			"DefaultVariant": {firm.Backed()},
			"VNC":            {firm.Backed()},

			"TmpfsMasks":    {firm.Elems[[]string](rule.ShowValue(firmrule.MaskDir))},
			"VolumeMasks":   {firm.Elems[[]string](rule.ShowValue(firmrule.MaskDir))},
			"ReadOnlyGlobs": {firm.Elems[[]string](rule.ShowValue(firmrule.MaskGlob))},

			// host dir → container mount dir, bound read-only
			"ReadOnlyBinds": {firm.KeyValues[map[string]string](firmrule.Bind{
				Specials: []string{GitConfigKey},
			})},

			"ForwardEnv": {firm.Elems[[]string](rule.ShowValue(firmrule.EnvVar))},
			"Proxy":      {firm.Backed()},
		}))
}

// defaultVariant is the default variant ask's section — the devbox with no run flags
type defaultVariant struct {
	// PortBindings binds tcp ports on localhost, container port = host port
	PortBindings *portBindings `yaml:"port_bindings"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[defaultVariant]().
		Validates(firm.RuleMap{
			"PortBindings": {firm.Backed()},
		}))
}

type portBindings struct {
	// TCP is the comma-separated ports to bind
	TCP string `yaml:"tcp"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[portBindings]().
		Validates(firm.RuleMap{
			"TCP": {firmrule.PortList{}},
		}))
}

// vnc is the desktop's session: the GUI app it installs and launches.
type vnc struct {
	// Enabled always runs with VNC served — the --vnc flag's config form
	Enabled *bool `yaml:"enabled,omitempty"`
	// GUIAppName is the GUI app's name (guiapp.Names()); empty means no GUI app
	GUIAppName string `yaml:"gui_app,omitempty"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[vnc]().
		Validates(firm.RuleMap{
			"GUIAppName": {rule.OneOf[string]{
				Values: append(guiapp.Names(), ""), // empty name means no GUI app to run
			}},
		}))
}

// proxy is the egress wall's section
type proxy struct {
	// Enabled runs with the egress wall
	Enabled *bool `yaml:"enabled,omitempty"`
	// Allowlist is the domains the egress wall lets through; each entry also matches
	// all its subdomains
	Allowlist []string `yaml:"allowlist"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[proxy]().
		Validates(firm.RuleMap{
			"Allowlist": {firm.Elems[[]string](rule.ShowValue(firmrule.DomainOrAlias(allowlistAliases)))},
		}))
}

// Init writes a starter .ccbox.yaml to projectDir
func Init(projectDir string) error {
	return initConfig(filepath.Join(projectDir, projectConfigFileName), config{})
}

// userSeedConfig is the user-level seed's data with ccbox defaults — referenced in tests
func userSeedConfig(cliName string) *config {
	return &config{
		CLIName:       new(cliName),
		ForwardEnv:    new([]string{"TERM", "COLORTERM"}),
		TmpfsMasks:    new([]string{DefaultsAlias}),
		VolumeMasks:   new([]string{DefaultsAlias}),
		ReadOnlyGlobs: new([]string{DefaultsAlias}),
		ReadOnlyBinds: new(map[string]string{GitConfigKey: firmrule.EnabledValue}),
		Proxy: &proxy{
			Enabled:   new(true),
			Allowlist: []string{DefaultsAlias, runtime.AllRuntimesAlias, SetHarnessAlias},
		},
	}
}

// SeedConfig safe-seeds the config with ccbox's own defaults, naming the seeded cli.
func SeedConfig(path, cliName string) error {
	return errs.Swallow(initConfig(path, *userSeedConfig(cliName)), seed.ErrExists)
}

func initConfig(path string, c config) error {
	body, err := c.renderTmpl()
	if err != nil {
		return err
	}
	return seed.File(path, body)
}

var (
	//go:embed ccbox.yaml.tmpl
	configTmplSrc string
	tmpl          = template.Must(template.New("ccbox.yaml").Funcs(template.FuncMap{
		"yaml":   yamlutil.Value,
		"indent": yamlutil.Indent,
	}).Parse(configTmplSrc))
)

// configTmpl wraps config with per-entry comments for the fields that have them; the template
// passes nil for the rest, whose yaml calls render bare
type configTmpl struct {
	config

	TmpfsMasksComments    []string
	VolumeMasksComments   []string
	ReadOnlyGlobsComments []string
	AllowlistComments     []string
	Allowlist             []string
}

func (c *config) renderTmpl() ([]byte, error) {
	var allowlist, allowlistComments []string
	if c.Proxy != nil {
		allowlist = c.Proxy.Allowlist
		allowlistComments = yamlutil.EntryToComments(c.Proxy.Allowlist, map[string]string{
			DefaultsAlias:            "shared tooling domains",
			runtime.AllRuntimesAlias: "every runtime's domains (node, python, go, rust)",
			SetHarnessAlias:          "the selected CLI's — and on --vnc, the GUI app's",
		})
	}
	var b bytes.Buffer
	data := configTmpl{
		config:                *c,
		TmpfsMasksComments:    yamlutil.CommentFirstEntry(ptrx.Deref(c.TmpfsMasks), defaultsExpansionComment(tmpfsDefaults)),
		VolumeMasksComments:   yamlutil.CommentFirstEntry(ptrx.Deref(c.VolumeMasks), defaultsExpansionComment(volumeDefaults)),
		ReadOnlyGlobsComments: yamlutil.CommentFirstEntry(ptrx.Deref(c.ReadOnlyGlobs), defaultsExpansionComment(readOnlyDefaults)),
		Allowlist:             allowlist,
		AllowlistComments:     allowlistComments,
	}
	if err := tmpl.Execute(&b, data); err != nil {
		return nil, fmt.Errorf("render Config template: %w", err)
	}
	return b.Bytes(), nil
}

func defaultsExpansionComment(defaults []string) string {
	if len(defaults) > 3 {
		return strings.Join(defaults[:3], ", ") + ", etc., see `ccbox config defaults` for expansions"
	}
	return strings.Join(defaults, ", ")
}

// Flags are the run flags Load folds into the config as its top layer — an unset
// flag is nil, keeping the files' layering
type Flags struct {
	CLI             *string // --cli: the coding CLI to install + launch
	VNC             bool    // --vnc: serve the desktop session
	NoProxy         bool    // --no-proxy: run without the egress wall — `ccbox run`'s own ask
	ProxyCmd        bool    // the proxy command's ask: the egress wall forced on
	PortBindingsTCP *string // --port-bindings-tcp: bind tcp ports on any variant ask
}

func (f Flags) toLayer() config {
	c := config{CLIName: f.CLI}
	if f.PortBindingsTCP != nil {
		c.DefaultVariant = &defaultVariant{PortBindings: &portBindings{TCP: *f.PortBindingsTCP}}
	}
	return c
}

// Load reads the layerPaths, the flags the top layer, and validates
func Load(projectDir string, flags Flags) (*Config, error) {
	layers, err := yamlutil.ReadLayers[config](layerPaths(projectDir))
	if err != nil {
		return nil, err
	}
	merged := yamlutil.MergeLayers(append(layers, flags.toLayer()), mergeempty.Merge[config])
	if errMap := firm.ValidateAny(merged); errMap != nil {
		return nil, errMap
	}
	s := merged.toInnerModel()
	s.projectDir = projectDir
	s.serveVNC = flags.VNC || s.VNC.Enabled
	s.serveProxied = flags.ProxyCmd || (!flags.NoProxy && s.Proxy.Enabled)

	portBindingsTCP := ""
	if s.isDefaultVariant() {
		portBindingsTCP = s.DefaultVariant.PortBindings.TCP
	}
	if flags.PortBindingsTCP != nil {
		portBindingsTCP = *flags.PortBindingsTCP
	}
	s.portBindingsTCP = splitPorts(portBindingsTCP)
	return &s, nil
}

// splitPorts splits the csv into its trimmed ports — infallible post-validation
func splitPorts(tcp string) []string {
	if tcp == "" {
		return nil
	}
	ports := strings.Split(tcp, ",")
	for i, entry := range ports {
		ports[i] = strings.TrimSpace(entry)
	}
	return ports
}
