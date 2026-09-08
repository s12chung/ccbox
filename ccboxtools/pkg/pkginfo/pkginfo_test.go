package pkginfo

import (
	"testing"

	"github.com/s12chung/firm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPkgInfo_JSON(t *testing.T) {
	tests := []struct {
		name string
		p    PkgInfo
		want string
	}{
		{
			"npm",
			PkgInfo{Name: "claude", Npm: &Npm{Package: "@anthropic-ai/claude-code"}},
			`{"name":"claude","npm":{"package":"@anthropic-ai/claude-code"},"versionurl":null}`,
		},
		{
			"versionurl",
			PkgInfo{Name: "grok", VersionURL: &VersionURL{
				URL:           "https://x.ai/cli/stable",
				LinuxX64URL:   "https://x.ai/cli/grok-$version-linux-x86_64",
				LinuxArm64URL: "https://x.ai/cli/grok-$version-linux-aarch64",
			}},
			`{"name":"grok","npm":null,"versionurl":{"url":"https://x.ai/cli/stable",` +
				`"linux_x64_url":"https://x.ai/cli/grok-$version-linux-x86_64",` +
				`"linux_arm64_url":"https://x.ai/cli/grok-$version-linux-aarch64"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := tt.p.JSON()
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, body)
		})
	}
}

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
				VersionURL: &VersionURL{URL: "https://x", LinuxX64URL: "https://x", LinuxArm64URL: "https://x"},
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
