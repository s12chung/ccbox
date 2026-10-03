package pkginfo

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVNCInfoFromJSON(t *testing.T) {
	tests := []struct {
		name string
		body string
		want VNCInfo
	}{
		{"no config defaults the resolution", `{}`, VNCInfo{Config: &VNCConfig{Resolution: DefaultResolution}}},
		{"empty config defaults the resolution", `{"config":{}}`, VNCInfo{Config: &VNCConfig{Resolution: DefaultResolution}}},
		{"config alone", `{"config":{"resolution":"1600x900"}}`, VNCInfo{Config: &VNCConfig{Resolution: "1600x900"}}},
		{
			"with gui app",
			`{"gui_app":{"name":"app","npm":{"package":"app"}},"config":{"resolution":"1600x900"}}`,
			VNCInfo{
				GUIApp: &GUIPkgInfo{PkgInfo: PkgInfo{Name: "app", Npm: &Npm{Package: "app"}}},
				Config: &VNCConfig{Resolution: "1600x900"},
			},
		},
		{
			"gui app with release_url and launch wiring",
			`{"gui_app":{"name":"zcode","version_url":{"url":"https://z",` +
				`"jq_schema":{"format":"yaml","version":".version","download_url":".url"},` +
				`"artifact":{"type":"deb","rel_bin":"opt/Z/z"}},"args":["--no-sandbox"],"desktop_name":"ZCode"},` +
				`"config":{"resolution":"1600x900"}}`,
			VNCInfo{
				GUIApp: &GUIPkgInfo{
					PkgInfo: PkgInfo{
						Name: "zcode",
						ReleaseURL: &ReleaseURL{
							URL:      "https://z",
							JQSchema: &JQSchema{Format: "yaml", Version: ".version", DownloadURL: ".url"},
							Artifact: &Artifact{Type: "deb", RelBin: "opt/Z/z"},
						},
					},
					Args:        []string{"--no-sandbox"},
					DesktopName: "ZCode",
				},
				Config: &VNCConfig{Resolution: "1600x900"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := VNCInfoFromJSON(tt.body)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestVNCInfoFromJSON_Invalid(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"unknown field", `{"config":{"resolution":"1600x900"},"depth":24}`, "unknown field"},
		{"unknown gui_app field", `{"gui_app":{"name":"app","icon":"z"},"config":{"resolution":"1600x900"}}`, "unknown field"},
		{"bad JSON", `{"config":`, "parse"},
		{"bad format", `{"config":{"resolution":"1600 by 900"}}`, "must be WxH"},
		{"below bounds", `{"config":{"resolution":"31x32"}}`, "within 32..16384"},
		{"above bounds", `{"config":{"resolution":"16385x900"}}`, "within 32..16384"},
		{"gui app without source", `{"gui_app":{"name":"app"},"config":{"resolution":"1600x900"}}`, "OneNotNil"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := VNCInfoFromJSON(tt.body)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestVNCInfoFromJSON_RoundTrip(t *testing.T) {
	// the host builds the env JSON from this type — it must parse back valid
	info := VNCInfo{
		GUIApp: &GUIPkgInfo{
			PkgInfo:     PkgInfo{Name: "app", Npm: &Npm{Package: "app"}},
			Args:        []string{"--no-sandbox"},
			DesktopName: "App",
		},
		Config: &VNCConfig{Resolution: "1600x900"},
	}
	body, err := json.Marshal(info)
	require.NoError(t, err)

	got, err := VNCInfoFromJSON(string(body))
	require.NoError(t, err)
	assert.Equal(t, info, got)
}
