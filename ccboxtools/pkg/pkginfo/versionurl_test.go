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
			"ok",
			VersionURL{
				URL:           "https://x.ai/cli/stable",
				LinuxX64URL:   "https://x.ai/cli/grok-$version-linux-x86_64",
				LinuxArm64URL: "https://x.ai/cli/grok-$version-linux-aarch64",
			},
			nil,
		},
		{
			"all empty",
			VersionURL{},
			[]string{"URL.Match", "LinuxX64URL.Match", "LinuxArm64URL.Match"},
		},
		{
			"non-https",
			VersionURL{
				URL:           "http://x.ai/cli/stable",
				LinuxX64URL:   "https://x.ai/cli/grok-$version-linux-x86_64",
				LinuxArm64URL: "https://x.ai/cli/grok-$version-linux-aarch64",
			},
			[]string{"URL.Match"},
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
