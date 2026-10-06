package pkginfo

import (
	"github.com/s12chung/firm"
)

// GUIPkgInfo describes a GUI app: its PkgInfo install source plus the session's
// launch wiring. It travels to the container as the VNCConfigEnvVar JSON's gui_app.
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
