package pkginfo

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustJSON(t *testing.T, g GUIPkgInfo) string {
	t.Helper()
	body, err := json.Marshal(g)
	require.NoError(t, err)
	return string(body)
}

func TestGUIPkgInfo_Validate(t *testing.T) {
	tests := []struct {
		name string
		g    GUIPkgInfo
		want string
	}{
		{"no install source", GUIPkgInfo{Args: []string{"--no-sandbox"}}, "OneNotNil"},
		{"path escape name", GUIPkgInfo{PkgInfo: PkgInfo{Name: "../evil", Npm: &Npm{Package: "a"}}}, "Name.Match"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GUIFromJSON(mustJSON(t, tt.g))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestGUIFromJSON(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		body := `{"name":"zcode","version_url":{"url":"https://z",` +
			`"jq_schema":{"format":"yaml","version":".version","download_url":".url"},` +
			`"artifact":{"type":"deb","rel_bin":"opt/Z/z"}},"args":["--no-sandbox"],"desktop_name":"ZCode"}`
		g, err := GUIFromJSON(body)
		require.NoError(t, err)
		assert.Equal(t, "zcode", g.Name)
		assert.Equal(t, []string{"--no-sandbox"}, g.Args)
		assert.Equal(t, "ZCode", g.DesktopName)
	})

	tests := []struct {
		name string
		body string
		want string
	}{
		{"unknown field", `{"name":"x","npm":{"package":"a"},"extra":1}`, "unknown field"},
		{"unknown gui field", `{"name":"x","npm":{"package":"a"},"icon":"z"}`, "unknown field"},
		{"bad json", `{`, "parse"},
		{"no install source", `{"name":"x","args":["--no-sandbox"]}`, "OneNotNil"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GUIFromJSON(tt.body)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}
