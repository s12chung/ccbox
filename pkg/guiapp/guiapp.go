// Package guiapp defines the image's GUI apps: the definition travels to the
// container inside the pkginfo.VNCConfigEnvVar JSON, while its download domains
// open the host's egress wall.
package guiapp

import (
	"fmt"
	"maps"
	"slices"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
)

// GUIApp is a GUI app the image ships: its definition plus the egress wall
// domains its download needs.
type GUIApp struct {
	pkginfo.GUIPkgInfo

	AllowDomains []string
}

// App is the image's GUI app (zcode): the vendor's electron release manifest,
// which requires the platform query (a bare URL is a parameter error).
var App = GUIApp{
	GUIPkgInfo: pkginfo.GUIPkgInfo{
		PkgInfo: pkginfo.PkgInfo{
			Name: "zcode",
			ReleaseURL: &pkginfo.ReleaseURL{
				// channel pins stable — the updater's preview channel is 3
				URL: "https://zcode.z.ai/api/v1/releases/electron/manifest?platform=linux-$arch&channel=1",
				JQSchema: &pkginfo.JQSchema{
					Format:      "yaml",
					Version:     ".version",
					DownloadURL: `.files[] | select(.url | endswith(".deb")) | .url`,
					Sha512:      `.files[] | select(.url | endswith(".deb")) | .sha512`,
				},
				Artifact: &pkginfo.Artifact{Type: "deb", RelBin: "opt/ZCode/zcode"},
			},
		},
		// electron's sandbox needs the user namespaces the container lacks
		Args:        []string{"--no-sandbox"},
		DesktopName: "ZCode",
	},
	// zcode.z.ai serves the version manifest; cdn-zcode.z.ai the deb itself
	AllowDomains: []string{"zcode.z.ai", "cdn-zcode.z.ai"},
}

// Apps maps the config's gui_app name to its definition
var Apps = map[string]GUIApp{App.Name: App}

// Names lists the config's gui_app choices
func Names() []string { return slices.Sorted(maps.Keys(Apps)) }

// For resolves the config's gui_app name to its definition
func For(name string) (GUIApp, error) {
	app, ok := Apps[name]
	if !ok {
		return GUIApp{}, fmt.Errorf("guiapp: %s is not one of %v", name, Names())
	}
	return app, nil
}

// AllowDomains resolves the named app's download domains — empty when the
// name names no app
func AllowDomains(name string) []string { return Apps[name].AllowDomains }
