package pkginfo

import (
	"testing"

	"github.com/s12chung/firm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionURL_Validate(t *testing.T) {
	tests := []struct {
		name string
		urls VersionURL
		want []string
	}{
		{
			"ok template",
			VersionURL{
				URL: "https://x.ai/cli/stable",
				DownloadTemplate: &DownloadTemplate{
					X64URL:   "https://x.ai/cli/grok-$version-linux-x86_64",
					Arm64URL: "https://x.ai/cli/grok-$version-linux-aarch64",
				},
			},
			nil,
		},
		{
			"no download_template",
			VersionURL{URL: "https://x.ai/cli/stable"},
			[]string{"DownloadTemplate.Nil"},
		},
		{
			"non-https url",
			VersionURL{
				URL:              "http://x.ai/cli/stable",
				DownloadTemplate: &DownloadTemplate{X64URL: "https://x", Arm64URL: "https://x"},
			},
			[]string{"URL.Match"},
		},
		{
			"non-https template",
			VersionURL{
				URL:              "https://x.ai/cli/stable",
				DownloadTemplate: &DownloadTemplate{X64URL: "ftp://x", Arm64URL: "https://x"},
			},
			[]string{"X64URL.Match"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMap := firm.ValidateAny(tt.urls)
			if tt.want == nil {
				assert.Nil(t, errMap.ToNil())
				return
			}
			require.NotEmpty(t, errMap)
			for _, want := range tt.want {
				assert.Contains(t, errMap.Error(), want)
			}
		})
	}
}
