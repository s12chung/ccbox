// Package guiapp describes the image's GUI app: the definition travels to the
// container as the pkginfo.GUIAppEnvVar JSON, while its download domains open
// the host's egress wall.
package guiapp

import (
	"encoding/json"
	"fmt"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
)

// App is the image's GUI app (zcode): the vendor's electron release manifest,
// which requires the platform query (a bare URL is a parameter error).
var App = pkginfo.GUIPkgInfo{
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
}

// AllowDomains are the app's download domains for the egress wall — host-only,
// the container downloads through the already-open wall. zcode.z.ai serves the
// version manifest; cdn-zcode.z.ai the deb itself.
var AllowDomains = []string{"zcode.z.ai", "cdn-zcode.z.ai"}

// JSON renders App for the container env.
func JSON() (string, error) {
	body, err := json.Marshal(App)
	if err != nil {
		return "", fmt.Errorf("guiapp: %w", err)
	}
	return string(body), nil
}
