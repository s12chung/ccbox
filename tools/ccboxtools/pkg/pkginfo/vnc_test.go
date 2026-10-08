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
		{"empty", `{}`, VNCInfo{}},
		{
			"with gui app",
			`{"gui_app":{"name":"app","npm":{"package":"app"}}}`,
			VNCInfo{GUIApp: &GUIPkgInfo{PkgInfo: PkgInfo{Name: "app", Npm: &Npm{Package: "app"}}}},
		},
		{
			"gui app with release_url and launch wiring",
			`{"gui_app":{"name":"zcode","version_url":{"url":"https://z",` +
				`"jq_schema":{"format":"yaml","version":".version","download_url":".url"},` +
				`"artifact":{"type":"deb","rel_bin":"opt/Z/z"}},"args":["--no-sandbox"],"desktop_name":"ZCode"}}`,
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
		{"unknown field", `{"depth":24}`, "unknown field"},
		{"unknown gui_app field", `{"gui_app":{"name":"app","icon":"z"}}`, "unknown field"},
		{"bad JSON", `{"gui_app":`, "parse"},
		{"gui app without source", `{"gui_app":{"name":"app"}}`, "OneNotNil"},
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
	}
	body, err := json.Marshal(info)
	require.NoError(t, err)

	got, err := VNCInfoFromJSON(string(body))
	require.NoError(t, err)
	assert.Equal(t, info, got)
}
