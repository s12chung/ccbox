package pkger

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
)

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

func TestVersionURL_Install(t *testing.T) {
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.URL.Path)
		_, _ = w.Write([]byte("#!/bin/sh\n"))
	}))
	defer srv.Close()

	u := VersionURL{
		VersionURL: pkginfo.VersionURL{
			URL:           srv.URL + "/stable",
			LinuxX64URL:   srv.URL + "/grok-$version-x64",
			LinuxArm64URL: srv.URL + "/grok-$version-arm64",
		},
		name: "grok",
	}

	dir := t.TempDir()
	require.NoError(t, u.Install(dir, "1.0.5"))

	// the raw binary lands at the dir root, executable
	//nolint:gosec // test fixture path
	body, err := os.ReadFile(filepath.Join(dir, "grok"))
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\n", string(body))
	info, err := os.Stat(filepath.Join(dir, "grok"))
	require.NoError(t, err)
	assert.Equal(t, execFileMode, info.Mode().Perm())

	// the running arch's template was hit with the version substituted
	want := "/grok-1.0.5-x64"
	if runtime.GOARCH == "arm64" {
		want = "/grok-1.0.5-arm64"
	}
	assert.Equal(t, []string{want}, hits)

	assert.Equal(t, "grok", u.RelBin())
}
