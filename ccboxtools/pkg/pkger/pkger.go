// Package pkger describes a coding CLI's install source — npm or a version URL —
// as the CLI_PKGER JSON the container consumes to install the CLI at start.
package pkger

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

// Pkger describes a CLI's install source: its name plus exactly one of Npm or
// VersionURL. It travels to the container as the CLI_PKGER env JSON.
type Pkger struct {
	Name       string      `json:"name"`
	Npm        *Npm        `json:"npm"`
	VersionURL *VersionURL `json:"versionurl"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[Pkger]().
		ValidatesSelf(rule.OneNotNil{Fields: []string{"Npm", "VersionURL"}}).
		Validates(firm.RuleMap{
			"Name":       {cliName},
			"Npm":        {firm.Backed()},
			"VersionURL": {firm.Backed()},
		}))
}

// JSON renders p as the CLI_PKGER JSON
func (p Pkger) JSON() (string, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("pkger: %w", err)
	}
	return string(body), nil
}

// FromJSON parses CLI_PKGER JSON, rejecting unknown fields and invalid values.
func FromJSON(body string) (Pkger, error) {
	var p Pkger
	dec := json.NewDecoder(strings.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return Pkger{}, fmt.Errorf("pkger: parse %s: %w", body, err)
	}
	if errMap := firm.ValidateAny(p); errMap != nil {
		return Pkger{}, fmt.Errorf("pkger: %w", errMap)
	}
	return p, nil
}
