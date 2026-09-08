// Package pkginfo describes a coding CLI's install source — npm or a version URL —
// as the CLI_PKGINFO JSON the container consumes to install the CLI at start.
package pkginfo

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"
)

// cliName is a filesystem-safe CLI name: no separators, no leading dot
var cliName = rule.Match{Regexp: regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)}

// EnvVar is the container env var carrying a PkgInfo's JSON.
const EnvVar = "CLI_PKGINFO"

// PkgInfo describes a CLI's install source: its name plus exactly one of Npm or
// VersionURL. It travels to the container as the CLI_PKGINFO env JSON.
type PkgInfo struct {
	Name       string      `json:"name"`
	Npm        *Npm        `json:"npm"`
	VersionURL *VersionURL `json:"versionurl"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[PkgInfo]().
		ValidatesSelf(rule.OneNotNil{Fields: []string{"Npm", "VersionURL"}}).
		Validates(firm.RuleMap{
			"Name":       {cliName},
			"Npm":        {firm.Backed()},
			"VersionURL": {firm.Backed()},
		}))
}

// JSON renders p as the CLI_PKGINFO JSON
func (p PkgInfo) JSON() (string, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("pkginfo: %w", err)
	}
	return string(body), nil
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
