// Package guiapp defines the image's GUI apps: the definition travels to the
// container inside the pkginfo.VNCConfigEnvVar JSON, while its download domains
// open the host's egress wall.
package guiapp

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"maps"
	"slices"
	"strconv"

	"github.com/s12chung/firm"

	"github.com/s12chung/ccbox/pkg/models/app"
	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

// GUIApp is a GUI app the image ships: its install definition plus its Setup.
type GUIApp struct {
	pkginfo.GUIPkgInfo

	// Setup is what ccbox sets up for the app in the container, shared with CLI's
	Setup app.Setup
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[GUIApp]().
		Validates(firm.RuleMap{
			"GUIPkgInfo": {firm.Backed()},
			"Setup":      {firm.Backed()},
		}))
}

// App is the image's GUI app (zcode): the vendor's electron release manifest,
// which requires the platform query (a bare URL is a parameter error).
var App = GUIApp{
	GUIPkgInfo: pkginfo.GUIPkgInfo{
		PkgInfo: pkginfo.PkgInfo{
			Name: "zcode",
			ReleaseURL: &pkginfo.ReleaseURL{
				// stable (1) lags bare requests (no device_mid); preview (3) ships buggy releases
				URL: "https://zcode.z.ai/api/v1/releases/electron/manifest?platform=linux-$arch&channel=1",
				JQSchema: &pkginfo.JQSchema{
					Format:      "yaml",
					Version:     ".version",
					DownloadURL: `.files[] | select(.url | endswith(".deb")) | .url`,
					Sha512: &pkginfo.Sha512Pin{
						Selector: `.files[] | select(.url | endswith(".deb")) | .sha512`,
						Encoding: "base64",
					},
				},
				Artifact: &pkginfo.Artifact{Type: "deb", RelBin: "opt/ZCode/zcode"},
			},
		},
		// electron's sandbox needs the user namespaces the container lacks
		Args:        []string{"--no-sandbox"},
		DesktopName: "ZCode",
	},
	Setup: app.Setup{
		// zcode 3.18.x defaults to the OS keyring, which the container lacks
		Env: map[string]string{
			"ZCODE_CREDENTIAL_SECRET_FILE": pkginfo.ContainerHome + "/.zcode/v2/credential-secret",
			// so the app's deep-link registration finds the seeded system-level entry
			"XDG_DATA_DIRS": pkginfo.ContainerHome + "/.ccbox/xdg/share:/usr/local/share:/usr/share",
		},
		DataBinds: map[string]*string{
			".zcode/v2":                   nil, // bind from the dir because the app causes EBUSY on files
			".zcode/v2/credential-secret": new(randomSecret()),
			".ccbox/xdg/share/applications/zcode.desktop": new(systemDesktopEntry),
		},
		Proxied: app.Proxied{
			// zcode.z.ai for the version manifest, cdn-zcode the deb; the rest is for oauth
			AllowDomains: []string{"zcode.z.ai", "cdn-zcode.z.ai", "chat.z.ai", "chatglm.cn", "aliyuncs.com", "alicdn.com", "google.com"},
			DataBinds: map[string]*string{
				".zcode/v2/setting.json": new(`{"httpProxy":` + strconv.Quote(proxyURL()) + `,"httpProxyNoProxy":"localhost,127.0.0.1,::1"}`),
			},
		},
	},
}

// Apps maps the config's gui_app name to its definition
var Apps = map[string]GUIApp{App.Name: App}

// Names lists the config's gui_app choices
func Names() []string { return slices.Sorted(maps.Keys(Apps)) }

// lookup resolves the config's gui_app name to its definition
func lookup(name string) (GUIApp, error) {
	guiApp, ok := Apps[name]
	if !ok {
		return GUIApp{}, fmt.Errorf("guiapp: %q is not one of %v", name, Names())
	}
	return guiApp, nil
}

// MustFor resolves a name already validated (cfg.Load rejects unknown
// gui_app values); it panics on an unknown name.
func MustFor(name string) *GUIApp {
	app, err := lookup(name)
	if err != nil {
		panic(err)
	}
	return &app
}

// systemDesktopEntry seeds the system-level zcode.desktop the app's deep-link
// registration defers to — without one, it rewrites the menu entry to launch
// the raw binary, which dies without --no-sandbox
const systemDesktopEntry = `[Desktop Entry]
Name=ZCode
Exec=/usr/local/bin/ccboxtools guiapp exec %U
Terminal=false
Type=Application
Icon=zcode
Categories=Development;
MimeType=x-scheme-handler/zcode;
`

// randomSecret is the app's file-backed secret — never rewritten, so stored
// credentials stay decryptable
func randomSecret() string {
	key := make([]byte, 32)
	// unreachable: reads from crypto/rand never fail
	must.Get(rand.Read(key))
	return base64.StdEncoding.EncodeToString(key)
}

func proxyURL() string {
	return "http://" + pkginfo.ProxyAddr + ":" + strconv.Itoa(pkginfo.ProxyPort)
}
