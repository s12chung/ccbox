package pkginfo

import (
	"testing"

	"github.com/s12chung/firm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPkgInfo_Validate(t *testing.T) {
	tests := []struct {
		name string
		p    PkgInfo
		want []string
	}{
		{"no install source", PkgInfo{Name: "claude"}, []string{"OneNotNil"}},
		{
			"both install sources",
			PkgInfo{
				Name:       "claude",
				Npm:        &Npm{Package: "a"},
				ReleaseURL: &ReleaseURL{URL: "https://x", DownloadTemplate: &DownloadTemplate{X64URL: "https://x", Arm64URL: "https://x"}},
			},
			[]string{"OneNotNil"},
		},
		{"path escape name", PkgInfo{Name: "../evil", Npm: &Npm{Package: "a"}}, []string{"Name.Match"}},
		{"empty name", PkgInfo{Npm: &Npm{Package: "a"}}, []string{"Name.Match"}},
		{"bad npm", PkgInfo{Name: "claude", Npm: &Npm{Package: "My CLI"}}, []string{"Package.Match"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMap := firm.ValidateAny(tt.p)
			require.NotEmpty(t, errMap)
			for _, want := range tt.want {
				assert.Contains(t, errMap.Error(), want)
			}
		})
	}
}

func TestPkgInfoFromJSON(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		p, err := FromJSON(`{"name":"claude","npm":{"package":"@anthropic-ai/claude-code"}}`)
		require.NoError(t, err)
		assert.Equal(t, PkgInfo{Name: "claude", Npm: &Npm{Package: "@anthropic-ai/claude-code"}}, p)
	})

	tests := []struct {
		name string
		body string
		want string
	}{
		{"unknown field", `{"name":"x","npm":{"package":"a"},"extra":1}`, "unknown field"},
		{"bad json", `{`, "parse"},
		{"no install source", `{"name":"x"}`, "OneNotNil"},
		{"path escape name", `{"name":"../x","npm":{"package":"a"}}`, "Name.Match"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FromJSON(tt.body)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestFromJSON_BadFormat(t *testing.T) {
	body := `{"name":"z","version_url":{"url":"https://z",` +
		`"jq_schema":{"format":"xml","version":".version","download_url":".url"}}}`
	_, err := FromJSON(body)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Format.OneOf")
}

func TestFromJSON_BadSelector(t *testing.T) {
	body := `{"name":"z","version_url":{"url":"https://z",` +
		`"jq_schema":{"format":"yaml","version":".version]","download_url":".url"}}}`
	_, err := FromJSON(body)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Version.JQExpr")
}
