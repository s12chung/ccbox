package pkger

import (
	"net/http"
	"net/http/httptest"
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

func TestVersion_At(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("  1.0.5\n"))
		}))
		defer srv.Close()

		v, err := versionAt(srv.URL)
		require.NoError(t, err)
		assert.Equal(t, "1.0.5", v)
	})

	for _, body := range []string{"", "not found", "<html>502</html>", "1.0.5\n1.0.6"} {
		t.Run("rejects "+body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()

			_, err := versionAt(srv.URL)
			assert.Error(t, err)
		})
	}
}
