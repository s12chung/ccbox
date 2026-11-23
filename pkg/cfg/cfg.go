// Package cfg loads the config file layers — user (~/.ccbox), project
// (repo root) — lowest precedence first
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
	"github.com/s12chung/ccbox/pkg/kit/yamlkit"
	"github.com/s12chung/ccbox/pkg/models/cli"
	"github.com/s12chung/ccbox/pkg/models/guiapp"
	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/errs"
	"github.com/s12chung/ccbox/pkg/util/mergeempty"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/ptrx"
	"github.com/s12chung/ccbox/pkg/util/seed"
	"github.com/s12chung/ccbox/pkg/util/slicex"
)

const (
	// userConfigFileName is the user-level config's name under Dir (no dot)
	userConfigFileName = "ccbox.yaml"
	// projectConfigFileName is the project-level config's name at the project repo root
	projectConfigFileName = ".ccbox.yaml"
	// DefaultsAlias listed in allowlist, expands in place to the shared tooling defaults
	DefaultsAlias = "ccbox-defaults"
	// SetHarnessAlias listed in allowlist, expands in place to the selected CLI's egress
	// domains — plus the GUI app's on a vnc load
	SetHarnessAlias = "ccbox-set-harness"
)

// UserConfigFile is the user-level config's path
func UserConfigFile() string { return filepath.Join(userdir.Dir(), userConfigFileName) }

// layerPaths lists the config file paths in load order: user, project
func layerPaths(projectDir string) []string {
	return []string{UserConfigFile(), filepath.Join(projectDir, projectConfigFileName)}
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
	Images         *images            `yaml:"images"`          // the devbox image's FROM refs
	DefaultVariant *defaultVariant    `yaml:"default_variant"` // the default variant ask's section — the devbox with no run flags
	HostGateway    *hostGateway       `yaml:"host_gateway"`    // the host tcp ports' section
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
		NotNil("CLIName", "Images").
		Validates(firm.RuleMap{
			"CLIName":        {rule.OneOf[string]{ValuesFunc: cli.Names}},
			"Images":         {firm.Backed()},
			"DefaultVariant": {firm.Backed()},
			"HostGateway":    {firm.Backed()},
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

// images is the devbox image's FROM refs
type images struct {
	// Base is the base stage's ref — debian's trixie snapshot slim
	Base string `yaml:"base"`
	// Mise is the mise stage's ref — ghcr.io/jdx/mise's release tag
	Mise string `yaml:"mise"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[images]().
		Validates(firm.RuleMap{
			"Base": {rule.ShowValue(firmrule.ImageRef)},
			"Mise": {rule.ShowValue(firmrule.ImageRef)},
		}))
}

// dropDisabled enforces that section or Enabled nils mean disabled
func (c *config) dropDisabled() *config {
	if c.VNC == nil || c.VNC.Enabled == nil || !*c.VNC.Enabled {
		c.VNC = nil
	}
	if c.Proxy == nil || c.Proxy.Enabled == nil || !*c.Proxy.Enabled {
		c.Proxy = nil
	}
	return c
}

var (
	//go:embed ccbox.yaml.tmpl
	configTmplSrc string
	tmpl          = template.Must(template.New("ccbox.yaml").Funcs(template.FuncMap{
		"yaml":   yamlkit.Value,
		"indent": yamlkit.Indent,
	}).Parse(configTmplSrc))
)

func (c *config) renderTmpl() ([]byte, error) {
	var allowlist, allowlistComments []string
	if c.Proxy != nil {
		allowlist = c.Proxy.Allowlist
		allowlistComments = yamlkit.EntryToComments(c.Proxy.Allowlist, map[string]string{
			DefaultsAlias:            "shared tooling domains",
			runtime.AllRuntimesAlias: "every runtime's domains (node, python, go, rust)",
			SetHarnessAlias:          "the selected CLI's — and on --vnc, the GUI app's",
		})
	}
	var b bytes.Buffer
	data := struct {
		config

		ReadOnlyGlobFlow  string
		AllowlistComments []string
		Allowlist         []string
	}{
		config:            *c,
		ReadOnlyGlobFlow:  yamlkit.Flow(ptrx.Deref(c.ReadOnlyGlobs), roGlobGroups),
		Allowlist:         allowlist,
		AllowlistComments: allowlistComments,
	}
	if err := tmpl.Execute(&b, data); err != nil {
		return nil, fmt.Errorf("render Config template: %w", err)
	}
	return b.Bytes(), nil
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
	//
	//toinnermodel:convert splitPorts
	TCP string `yaml:"tcp"`
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

func init() {
	firm.MustRegisterType(firm.NewDefinition[portBindings]().
		Validates(firm.RuleMap{
			"TCP": {firmrule.PortList{}},
		}))
}

// hostGateway is the host tcp ports' section — what the devbox may reach on the host
type hostGateway struct {
	// TCP is the comma-separated host ports the devbox may reach at host.docker.internal
	//
	//toinnermodel:convert splitPorts
	TCP string `yaml:"tcp"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[hostGateway]().
		Validates(firm.RuleMap{
			"TCP": {firmrule.PortList{}},
		}))
}

// vnc is the vnc section
type vnc struct {
	// Enabled always runs with VNC served
	//
	//toinnermodel:skip
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
	//
	//toinnermodel:skip
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

// the seeded config's guardMount lists
var (
	tmpfsDefaults  = []string{".idea", ".vscode", ".zed"}
	volumeDefaults = []string{"node_modules", ".venv", "target"}

	roGlobDefaults = []string{".ccbox.yaml", "secrets", ".env", ".env.*", ".envrc", "**/*.pem", "**/*.key", "**/id_rsa*"}
	// roGlobGroups rows the seed's globs together at render
	roGlobGroups = map[string]int{
		".ccbox.yaml": 0, "secrets": 0,
		".env": 1, ".env.*": 1, ".envrc": 1,
		"**/*.pem": 2, "**/*.key": 2, "**/id_rsa*": 2,
	}
)

// userSeedConfig is the user-level seed's data with ccbox defaults — referenced in tests
func userSeedConfig(cliName string, refs Images) *config {
	return &config{
		CLIName:       new(cliName),
		Images:        new(images(refs)),
		ForwardEnv:    new([]string{"TERM", "COLORTERM"}),
		TmpfsMasks:    new(tmpfsDefaults),
		VolumeMasks:   new(volumeDefaults),
		ReadOnlyGlobs: new(roGlobDefaults),
		ReadOnlyBinds: new(map[string]string{GitConfigKey: firmrule.EnabledValue}),
		Proxy: &proxy{
			Enabled:   new(true),
			Allowlist: []string{DefaultsAlias, runtime.AllRuntimesAlias, SetHarnessAlias},
		},
	}
}

// Init writes a starter .ccbox.yaml to projectDir
func Init(projectDir string) error {
	return initConfig(filepath.Join(projectDir, projectConfigFileName), config{})
}

// SeedConfig safe-seeds the config with ccbox's own defaults, naming the seeded cli
// and pinning the images' refs.
func SeedConfig(path, cliName string, refs Images) error {
	return errs.Swallow(initConfig(path, *userSeedConfig(cliName, refs)), seed.ErrExists)
}

func initConfig(path string, c config) error {
	body, err := c.renderTmpl()
	if err != nil {
		return err
	}
	return seed.File(path, body)
}

// Flags are the run flags Load folds into the config as its top layer — an unset
// flag is nil, keeping the files' layering
type Flags struct {
	CLI             *string // --cli: the coding CLI to install + launch
	VNC             bool    // --vnc: serve the desktop session
	NoProxy         bool    // --no-proxy: run without the egress wall — `ccbox run`'s own ask
	ProxyCmd        bool    // the proxy command's ask: the egress wall forced on
	PortBindingsTCP *string // --port-bindings-tcp: bind tcp ports on any variant ask
	HostGatewayTCP  *string // --host-gateway-tcp: host ports the devbox may reach, on any ask
}

func (f Flags) toLayer() config {
	c := config{CLIName: f.CLI}
	if f.ProxyCmd {
		c.Proxy = &proxy{Enabled: new(true)}
	} else if f.NoProxy {
		c.Proxy = &proxy{Enabled: new(false)}
	}
	if f.VNC {
		c.VNC = &vnc{Enabled: new(true)}
	}
	if f.PortBindingsTCP != nil {
		c.DefaultVariant = &defaultVariant{PortBindings: &portBindings{TCP: *f.PortBindingsTCP}}
	}
	if f.HostGatewayTCP != nil {
		c.HostGateway = &hostGateway{TCP: *f.HostGatewayTCP}
	}
	return c
}

// Load reads the layerPaths, the flags the top layer, and validates
func Load(projectDir string, flags Flags) (*Config, error) {
	layers, err := yamlkit.ReadLayers[config](layerPaths(projectDir))
	if err != nil {
		return nil, err
	}
	filesConfig := yamlkit.MergeLayers(layers, mergeempty.Merge[config])

	merged := yamlkit.MergeLayers([]config{filesConfig, flags.toLayer()}, mergeempty.Merge[config])
	if errMap := firm.ValidateAny(merged); errMap != nil {
		return nil, errMap
	}

	s := merged.dropDisabled().toInnerModel()
	s.projectDir = projectDir
	// repeats across layers would remount the same path twice
	s.TmpfsMasks, s.VolumeMasks, s.ReadOnlyGlobs = slicex.Unique(s.TmpfsMasks), slicex.Unique(s.VolumeMasks), slicex.Unique(s.ReadOnlyGlobs)
	if flags.PortBindingsTCP == nil {
		if !s.isDefaultVariant(filesConfig.dropDisabled().toInnerModel()) {
			s.DefaultVariant = nil
		}
	}
	return &s, nil
}
