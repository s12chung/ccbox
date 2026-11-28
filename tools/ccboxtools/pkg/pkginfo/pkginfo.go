// Package pkginfo is the host's env contract with the container: an install
// source — a coding CLI or the GUI app — and the run's config asks.
package pkginfo

import (
	"regexp"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/errs"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/jsonutil"
)

const (
	// EnvVar is the container env var carrying a coding CLI's PkgInfo JSON.
	EnvVar = "CLI_PKGINFO"

	// HostGatewayHost is the /etc/hosts name resolving to the host gateway — the devbox's
	// address for the host's own services
	HostGatewayHost = "host.docker.internal"

	// ContainerHome is the container user's home: the mount targets' root within the container.
	ContainerHome = "/home/ccbox"

	// ProxyMount is the host's live proxy dir, bound read-only into the devbox
	ProxyMount = "/etc/ccbox/proxy"
	// ProxyAllowFile is the allow file's name — the one tinyproxy's baked conf points at
	ProxyAllowFile = "allow.txt"
	// ProxyAllowPath is the allow file's path in the devbox — the allowlist command's input.
	ProxyAllowPath = ProxyMount + "/" + ProxyAllowFile

	// ProxyAddr is the fixed address tinyproxy listens on — the target of the
	// devbox's proxy env and the room's default route.
	ProxyAddr = "169.254.100.1"
	// ProxyPort is tinyproxy's fixed listen port, shared with the proxy env.
	ProxyPort = 8888
	// ProxyMuxName is tinyproxy's tag in the container's shared log stream.
	ProxyMuxName = "tinyproxy"

	// VNCPort is the desktop's fixed RFB listen — the bare server's start gate.
	VNCPort = 5900

	// ReadyMarker is the boot's readiness line — the CLI install and the
	// desktop serve are done, so sessions may land
	ReadyMarker = "devbox: ready"
)

// LatestVersion is the version ask tracking each release.
const LatestVersion = "latest"

// versionRe is an exact version — 1.2.3, 2.0.0-beta.1
var versionRe = regexp.MustCompile(`^\d+(\.\d+)*([-+].+)?$`)

// IsPinned reports whether the version ask is an exact version.
func IsPinned(version string) bool { return versionRe.MatchString(version) }

// VersionRule accepts a version ask: "latest" or an exact version — dist-tags
// and ranges are rejected.
var VersionRule = rule.Or(rule.Equal[string]{To: LatestVersion}, rule.Match{Regexp: versionRe})

// PkgInfo describes a CLI's install source: its name plus exactly one of Npm or
// ReleaseURL. It travels to the container as the CLI_PKGINFO env JSON, and is
// inlined into cli.CLI's CLI.yaml.
type PkgInfo struct {
	Name       string      `json:"name"        yaml:"name"`
	Npm        *Npm        `json:"npm"         yaml:"npm"`
	ReleaseURL *ReleaseURL `json:"version_url" yaml:"version_url"`
}

// Npm installs from the npm registry's "latest" dist-tag.
type Npm struct {
	Package string `json:"package" yaml:"package"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[PkgInfo]().
		ValidatesSelf(rule.OneNotNil{Fields: []string{"Npm", "ReleaseURL"}}).
		Validates(firm.RuleMap{
			"Name":       {rule.Match{Regexp: regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)}},
			"Npm":        {firm.Backed()},
			"ReleaseURL": {firm.Backed()},
		}))
	firm.MustRegisterType(firm.NewDefinition[Npm]().Validates(firm.RuleMap{
		// npm package name, scoped or not, e.g. @anthropic-ai/claude-code
		"Package": {rule.Match{Regexp: regexp.MustCompile(`^(@[a-z0-9-]+/)?[a-z0-9][a-z0-9._-]*$`)}},
	}))
}

// FromJSON parses CLI_PKGINFO JSON, rejecting unknown fields and invalid values.
func FromJSON(body string) (PkgInfo, error) {
	pkgInfo, err := jsonutil.Decode[PkgInfo](body)
	return pkgInfo, errs.Prefix("pkginfo", err)
}
