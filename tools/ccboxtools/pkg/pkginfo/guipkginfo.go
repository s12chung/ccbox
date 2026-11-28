package pkginfo

import (
	"github.com/s12chung/firm"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/errs"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/jsonutil"
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

// VNCConfigEnvVar is the container env var carrying the desktop's VNCInfo JSON.
const VNCConfigEnvVar = "VNC_CONFIG"

// VNCInfo is the desktop's VNC session: the GUI app it installs and launches.
// It travels to the container as the VNCConfigEnvVar JSON.
type VNCInfo struct {
	// GUIApp is the session's GUI app; nil names none
	GUIApp *GUIPkgInfo `json:"gui_app,omitempty"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[VNCInfo]().
		Validates(firm.RuleMap{
			"GUIApp": {firm.Backed()},
		}))
}

// VNCInfoFromJSON parses VNCConfigEnvVar JSON, rejecting unknown fields and
// invalid values.
func VNCInfoFromJSON(body string) (VNCInfo, error) {
	info, err := jsonutil.Decode[VNCInfo](body)
	return info, errs.Prefix("pkginfo", err)
}
