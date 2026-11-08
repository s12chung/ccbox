// Package pkginfo describes an install source — a coding CLI or the GUI app —
// as the env JSON the container consumes to install it at start.
package pkginfo

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"
)

const (
	// EnvVar is the container env var carrying a coding CLI's PkgInfo JSON.
	EnvVar = "CLI_PKGINFO"

	// ProxyMount is the container's binded read-only dir for the proxy
	ProxyMount = "/etc/ccbox/proxy"
	// ProxyAllowFile tinyproxy's baked conf points at allow file
	ProxyAllowFile = "allow.txt"
	// ProxyAllowPath is ProxyAllowFile's devbox path: the allowlist command's input.
	ProxyAllowPath = ProxyMount + "/" + ProxyAllowFile

	// ProxyAddr is the fixed address tinyproxy listens on — the target of the
	// devbox's proxy env and the room's default route.
	ProxyAddr = "169.254.100.1"
	// ProxyPort is tinyproxy's fixed listen port, shared with the proxy env.
	ProxyPort = 8888
	// ProxyMuxName is tinyproxy's tag under in the container's shared log stream.
	ProxyMuxName = "tinyproxy"

	// ReadyMarker is the boot's readiness line with the egress wall on
	ReadyMarker = "egress wall: ready"
	// DevboxReadyMarker is the no-proxy boot's readiness line: the CLI install
	// and the desktop start are done, so sessions may land
	DevboxReadyMarker = "devbox: ready"
)

// PkgInfo describes a CLI's install source: its name plus exactly one of Npm or
// ReleaseURL. It travels to the container as the CLI_PKGINFO env JSON, and is
// inlined into cli.CLI's CLI.yaml.
type PkgInfo struct {
	Name       string      `json:"name"        yaml:"name"`
	Npm        *Npm        `json:"npm"         yaml:"npm"`
	ReleaseURL *ReleaseURL `json:"version_url" yaml:"version_url"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[PkgInfo]().
		ValidatesSelf(rule.OneNotNil{Fields: []string{"Npm", "ReleaseURL"}}).
		Validates(firm.RuleMap{
			"Name":       {rule.Match{Regexp: regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)}},
			"Npm":        {firm.Backed()},
			"ReleaseURL": {firm.Backed()},
		}))
}

// FromJSON parses CLI_PKGINFO JSON, rejecting unknown fields and invalid values.
func FromJSON(body string) (PkgInfo, error) {
	var p PkgInfo
	dec := json.NewDecoder(strings.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return PkgInfo{}, fmt.Errorf("pkginfo: parse %s: %w", body, err)
	}
	if errMap := firm.ValidateAny(p); errMap != nil {
		return PkgInfo{}, fmt.Errorf("pkginfo: %w", errMap)
	}
	return p, nil
}
