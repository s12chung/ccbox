package pkger

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionURLArg(t *testing.T) {
	u := VersionURL{
		URL:           "https://x.ai/cli/stable",
		LinuxX64URL:   "https://x.ai/cli/grok-$version-linux-x86_64",
		LinuxArm64URL: "https://x.ai/cli/grok-$version-linux-aarch64",
	}
	assert.Equal(t,
		"versionurl:https://x.ai/cli/stable|https://x.ai/cli/grok-$version-linux-x86_64|https://x.ai/cli/grok-$version-linux-aarch64",
		u.Arg())
}

func TestVersionURLLatest(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Write([]byte("  1.0.5\n"))
		}))
		defer srv.Close()

		v, err := versionAt(srv.URL)
		require.NoError(t, err)
		assert.Equal(t, "1.0.5", v)
	})

	for _, body := range []string{"", "not found", "<html>502</html>", "1.0.5\n1.0.6"} {
		t.Run("rejects "+body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Write([]byte(body))
			}))
			defer srv.Close()

			_, err := versionAt(srv.URL)
			assert.Error(t, err)
		})
	}
}
