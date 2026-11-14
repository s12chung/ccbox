package cfg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/gobwas/glob"

	"github.com/s12chung/ccbox/pkg/cfg/expand"
	"github.com/s12chung/ccbox/pkg/kit/git"
	"github.com/s12chung/ccbox/pkg/kit/globkit"
	"github.com/s12chung/ccbox/pkg/models/cli"
	"github.com/s12chung/ccbox/pkg/models/guiapp"
	"github.com/s12chung/ccbox/pkg/models/provider"
	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/mapx"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

// The read_only_binds' gitconfig special entry: the key carries the host's XDG git config
// dir, and the value mounts it at its default container path
const (
	// GitConfigKey is read_only_binds' special key: the host's XDG git config dir
	GitConfigKey = "gitconfig"
	// GitConfigMount is the gitconfig entry's container mount (git's default XDG path)
	GitConfigMount = pkginfo.ContainerHome + "/.config/git"
)

// Config is the loaded .ccbox.yaml
type Config struct {
	CLIName       string            // coding CLI to install + launch
	VNC           VNC               // the desktop's session: its GUI app
	TmpfsMasks    []string          // project-relative dirs to mask with a writable tmpfs
	VolumeMasks   []string          // project-relative dirs to mask with a persistent per-project volume
	ReadOnlyGlobs []string          // project-relative globs to re-mount read-only
	ReadOnlyBinds map[string]string // host dir → container mount dir, bound read-only
	ForwardEnv    []string          // host env vars forwarded into the container
	Proxy         Proxy             // the egress wall's section

	// projectDir anchors the masks' present-filters
	projectDir string
	// serveVNC is the resolved run mode — the --vnc flag or the vnc section's enabled;
	// only it gates the GUI app's domains into SetHarnessAlias's expansion
	serveVNC bool
	// cache the DefaultsAlias expansions of raw lists and the read_only_binds' path resolutions
	expandedTmpfsMasks    []string
	expandedVolumeMasks   []string
	expandedReadOnlyGlobs []string
	expandedReadOnlyBinds map[string]string
	expandedAllowlist     []string
}

// VNC is the desktop's session: the GUI app it installs and launches.
type VNC struct {
	// Enabled always runs with VNC served — the --vnc flag's config form
	Enabled bool
	// GUIAppName is the GUI app's name (guiapp.Names()); empty means no GUI app
	GUIAppName string
}

// IsEnabled reports whether the session runs with VNC served
func (v *VNC) IsEnabled() bool { return v.Enabled }

// Proxy is the egress wall's section — the seeded file pins its enabled default.
type Proxy struct {
	// Enabled runs with the egress wall
	Enabled bool
	// Allowlist is the domains the egress wall lets through; each entry also matches
	// all its subdomains
	Allowlist []string
}

// IsEnabled reports whether the egress wall runs
func (p *Proxy) IsEnabled() bool { return p.Enabled }

// IsProxied resolves the run's egress wall: the exec mode's proxied ask per the
// section's enabled — a vnc run proxies exactly like a headless one
func (p *Proxy) IsProxied(flag bool) bool { return flag && p.IsEnabled() }

// ProjectDir is the project dir the config was loaded from
func (c *Config) ProjectDir() string { return c.projectDir }

// ServeVNC is the resolved run mode: the --vnc flag or the vnc section's enabled —
// the flag alone acts as an enabled section with no app
func (c *Config) ServeVNC() bool { return c.serveVNC }

// CLI resolves the config's cli name to its cli.CLI; MustFor is
// infallible for a loaded config
func (c *Config) CLI() cli.CLI { return cli.MustFor(c.CLIName) }

// GUIApp resolves the run's GUI app: nil on a headless run or a vnc section naming
// no gui_app; MustFor is infallible for a loaded config
func (c *Config) GUIApp() *guiapp.GUIApp {
	if !c.ServeVNC() {
		return nil
	}
	return guiapp.MustFor(c.VNC.GUIAppName)
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
	return osutil.DirsPresent(c.projectDir, c.TmpfsMasksExpanded())
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
	return osutil.DirsPresent(c.projectDir, c.VolumeMasksExpanded())
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
// paths or more
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
				return osutil.ExpandHome(key, userdir.MustHome()), osutil.ExpandHome(mount, pkginfo.ContainerHome)
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

// AllowlistExpanded expands Proxy.Allowlist's aliases in place
func (c *Config) AllowlistExpanded() []string {
	if c.expandedAllowlist == nil {
		expansions := map[string][]string{
			DefaultsAlias:   AllowlistDefaults(),
			SetHarnessAlias: c.SetHarnessDomains(),
		}
		for _, alias := range slices.Concat(provider.Aliases(), runtime.Aliases()) {
			expansions[alias] = expandDomain(alias)
		}
		c.expandedAllowlist = expand.Slice(c.Proxy.Allowlist, expansions)
	}
	return c.expandedAllowlist
}

// SetHarnessDomains is what SetHarnessAlias expands to on the config: the selected
// CLI's egress domains, its provider aliases expanded, plus the GUI app's download
// domains on a vnc load — only a desktop run downloads the app.
func (c *Config) SetHarnessDomains() []string {
	var domains []string
	for _, d := range c.CLI().Setup.Proxied.AllowDomains {
		domains = append(domains, expandDomain(d)...)
	}
	if c.ServeVNC() {
		domains = append(domains, guiapp.AllowDomains(c.VNC.GUIAppName)...)
	}
	return domains
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

// MarshalYAML renders the effective config back in the input view's yaml-facing
// shape — what `ccbox config` prints.
func (c *Config) MarshalYAML() (any, error) {
	c.Proxy.Allowlist = c.AllowlistExpanded()
	return config{
		CLIName:       new(c.CLIName),
		VNC:           &vnc{Enabled: new(c.VNC.Enabled), GUIAppName: c.VNC.GUIAppName},
		TmpfsMasks:    c.TmpfsMasksPresent(),
		VolumeMasks:   c.VolumeMasksPresent(),
		ReadOnlyGlobs: c.ReadOnlyPathsPresent(),
		ReadOnlyBinds: c.ReadOnlyBindsPresent(),
		ForwardEnv:    c.ForwardEnv,
		Proxy:         &proxy{Enabled: new(c.Proxy.Enabled), Allowlist: c.Proxy.Allowlist},
	}, nil
}

// InfoJSON renders the section as the VNCConfigEnvVar JSON: the GUI app the run
// installs and launches
func (v *VNC) InfoJSON() (string, error) {
	var info pkginfo.VNCInfo
	if app := guiapp.MustFor(v.GUIAppName); app != nil {
		info.GUIApp = &app.GUIPkgInfo
	}
	body, err := json.Marshal(info)
	if err != nil {
		return "", fmt.Errorf("vnc config: %w", err)
	}
	return string(body), nil
}

func absentDirs(projectDir string, dirs []string) []string {
	present := osutil.DirsPresent(projectDir, dirs)
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
	return mapx.NilIfEmpty(present)
}

// absentKeys filters binds to the keys missing from present
func absentKeys(binds, present map[string]string) map[string]string {
	absent := make(map[string]string, len(binds))
	for key, value := range binds {
		if _, ok := present[key]; !ok {
			absent[key] = value
		}
	}
	return mapx.NilIfEmpty(absent)
}
