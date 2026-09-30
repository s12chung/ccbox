package pkginfo

import (
	"runtime"
	"testing"

	"github.com/s12chung/firm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReleaseURL_ArchedURL(t *testing.T) {
	arch := "x64"
	if runtime.GOARCH == "arm64" {
		arch = "arm64"
	}

	r := ReleaseURL{URL: "https://x.ai/manifest?platform=linux-$arch"}
	assert.Equal(t, "https://x.ai/manifest?platform=linux-"+arch, r.ArchedURL())

	r.URL = "https://x.ai/manifest"
	assert.Equal(t, r.URL, r.ArchedURL())
}

func TestReleaseURL_Validate(t *testing.T) {
	tests := []struct {
		name string
		urls ReleaseURL
		want []string
	}{
		{
			"ok template",
			ReleaseURL{
				URL: "https://x.ai/cli/stable",
				DownloadTemplate: &DownloadTemplate{
					X64URL:   "https://x.ai/cli/grok-$version-linux-x86_64",
					Arm64URL: "https://x.ai/cli/grok-$version-linux-aarch64",
				},
			},
			nil,
		},
		{
			"ok manifest deb",
			ReleaseURL{
				URL: "https://zcode.z.ai/manifest/linux-$arch",
				JQSchema: &JQSchema{
					Format:      "yaml",
					Version:     ".version",
					DownloadURL: `.files[] | select(.url | endswith(".deb")) | .url`,
					Sha512:      `.files[] | select(.url | endswith(".deb")) | .sha512`,
				},
				Artifact: &Artifact{Type: "deb", RelBin: "opt/ZCode/zcode"},
			},
			nil,
		},
		{
			"jq with template",
			ReleaseURL{
				URL:              "https://zcode.z.ai/manifest",
				DownloadTemplate: &DownloadTemplate{X64URL: "https://x", Arm64URL: "https://x"},
				JQSchema:         &JQSchema{Format: "json", Version: ".path"},
			},
			[]string{"OneNotNil"},
		},
		{
			"sha512 with template",
			ReleaseURL{
				URL:              "https://x.ai/cli/stable",
				DownloadTemplate: &DownloadTemplate{X64URL: "https://x", Arm64URL: "https://x"},
				JQSchema:         &JQSchema{Format: "yaml", Version: ".version", Sha512: ".sha512"},
			},
			[]string{"OneNotNil"},
		},
		{
			"no download source",
			ReleaseURL{
				URL: "https://zcode.z.ai/manifest",
			},
			[]string{"OneNotNil"},
		},
		{
			"template-nil-without-download_url",
			ReleaseURL{
				URL:      "https://zcode.z.ai/manifest",
				JQSchema: &JQSchema{Format: "yaml", Version: ".version"},
			},
			[]string{"DownloadURL.Present"},
		},
		{
			"non-https url",
			ReleaseURL{
				URL:              "http://x.ai/cli/stable",
				DownloadTemplate: &DownloadTemplate{X64URL: "https://x", Arm64URL: "https://x"},
			},
			[]string{"URL.Match"},
		},
		{
			"non-https template",
			ReleaseURL{
				URL:              "https://x.ai/cli/stable",
				DownloadTemplate: &DownloadTemplate{X64URL: "ftp://x", Arm64URL: "https://x"},
			},
			[]string{"X64URL.Match"},
		},
		{
			"unknown artifact type",
			ReleaseURL{
				URL:              "https://zcode.z.ai/manifest",
				DownloadTemplate: &DownloadTemplate{X64URL: "https://x", Arm64URL: "https://x"},
				Artifact:         &Artifact{Type: "rpm", RelBin: "opt/ZCode/zcode"},
			},
			[]string{"Type.OneOf"},
		},
		{
			"artifact-without-rel_bin",
			ReleaseURL{
				URL:              "https://zcode.z.ai/manifest",
				DownloadTemplate: &DownloadTemplate{X64URL: "https://x", Arm64URL: "https://x"},
				Artifact:         &Artifact{Type: "deb"},
			},
			[]string{"RelBin.Present"},
		},
		{
			"bad jq format",
			ReleaseURL{
				URL:              "https://zcode.z.ai/manifest",
				DownloadTemplate: &DownloadTemplate{X64URL: "https://x", Arm64URL: "https://x"},
				JQSchema:         &JQSchema{Format: "xml", Version: ".version"},
			},
			[]string{"Format.OneOf"},
		},
		{
			"empty jq version",
			ReleaseURL{
				URL:              "https://zcode.z.ai/manifest",
				DownloadTemplate: &DownloadTemplate{X64URL: "https://x", Arm64URL: "https://x"},
				JQSchema:         &JQSchema{Format: "yaml"},
			},
			[]string{"Version.Present"},
		},
		{
			"bad jq selector",
			ReleaseURL{
				URL:              "https://zcode.z.ai/manifest",
				DownloadTemplate: &DownloadTemplate{X64URL: "https://x", Arm64URL: "https://x"},
				JQSchema:         &JQSchema{Format: "yaml", Version: ".version]"},
			},
			[]string{"Version.JQExpr"},
		},
		{
			"bad download_url selector",
			ReleaseURL{
				URL:      "https://zcode.z.ai/manifest",
				JQSchema: &JQSchema{Format: "yaml", Version: ".version", DownloadURL: ".files["},
			},
			[]string{"DownloadURL.JQExpr"},
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
