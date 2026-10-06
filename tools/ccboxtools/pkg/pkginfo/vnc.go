package pkginfo

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/s12chung/firm"
)

// VNCConfigEnvVar is the container env var carrying the desktop's VNCInfo JSON.
const VNCConfigEnvVar = "VNC_CONFIG"

// DefaultResolution is the vnc resolution set when the config omits one — what
// the desktop script itself falls back to
const DefaultResolution = "1280x1024"

// VNCInfo is the desktop's VNC session: the GUI app it installs and launches
// plus its VNC config. It travels to the container as the VNCConfigEnvVar JSON.
type VNCInfo struct {
	// GUIApp is the session's GUI app; nil names none
	GUIApp *GUIPkgInfo `json:"gui_app,omitempty"`
	// Config is the session's VNC config; nil defaults on load
	Config *VNCConfig `json:"config,omitempty"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[VNCInfo]().
		Validates(firm.RuleMap{
			"GUIApp": {firm.Backed()},
			"Config": {firm.Backed()},
		}))
}

// VNCConfig is the desktop's VNC session config
type VNCConfig struct {
	// Resolution is the box's WxH, for VNC clients that cannot ask for a resize
	// themselves (e.g. macOS Screen Sharing)
	Resolution string `json:"resolution,omitempty" yaml:"resolution"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[VNCConfig]().
		Validates(firm.RuleMap{"Resolution": {wxh{}}}))
}

// Defaulted sets the zero fields' defaults: a nil config and an empty resolution
// both take DefaultResolution
func (v *VNCInfo) Defaulted() {
	if v.Config == nil {
		v.Config = &VNCConfig{}
	}
	if v.Config.Resolution == "" {
		v.Config.Resolution = DefaultResolution
	}
}

// VNCInfoFromJSON parses VNCConfigEnvVar JSON, rejecting unknown fields and
// invalid values; zero fields are defaulted.
func VNCInfoFromJSON(body string) (VNCInfo, error) {
	var v VNCInfo
	dec := json.NewDecoder(strings.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return VNCInfo{}, fmt.Errorf("pkginfo: parse %s: %w", body, err)
	}
	v.Defaulted()
	if errMap := firm.ValidateAny(v); errMap != nil {
		return VNCInfo{}, fmt.Errorf("pkginfo: %w", errMap)
	}
	return v, nil
}
