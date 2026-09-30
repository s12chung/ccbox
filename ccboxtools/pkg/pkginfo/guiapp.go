package pkginfo

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/s12chung/firm"
)

// GUIAppEnvVar is the container env var carrying the GUI app's GUIPkgInfo JSON.
const GUIAppEnvVar = "GUIAPP_PKGINFO"

// GUIPkgInfo describes a GUI app: its PkgInfo install source plus the session's
// launch wiring. It travels to the container as the GUIAppEnvVar JSON.
type GUIPkgInfo struct {
	PkgInfo

	// Args prepend the app's own argv at launch — electron apps need
	// --no-sandbox for the user namespaces the container lacks
	Args []string `json:"args,omitempty"`
	// DesktopName is the session menu entry's display name; empty generates no menu item
	DesktopName string `json:"desktop_name,omitempty"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[GUIPkgInfo]().
		Validates(firm.RuleMap{"PkgInfo": {firm.Backed()}}))
}

// GUIFromJSON parses GUIAppEnvVar JSON, rejecting unknown fields and invalid values.
func GUIFromJSON(body string) (GUIPkgInfo, error) {
	var g GUIPkgInfo
	dec := json.NewDecoder(strings.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&g); err != nil {
		return GUIPkgInfo{}, fmt.Errorf("pkginfo: parse %s: %w", body, err)
	}
	if errMap := firm.ValidateAny(g); errMap != nil {
		return GUIPkgInfo{}, fmt.Errorf("pkginfo: %w", errMap)
	}
	return g, nil
}
