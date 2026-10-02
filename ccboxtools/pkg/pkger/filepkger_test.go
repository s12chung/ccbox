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

func TestFilePkger_Install(t *testing.T) {
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.URL.Path)
		_, _ = w.Write([]byte("#!/bin/sh\n"))
	}))
	defer srv.Close()

	p := FilePkger{
		Downloader: VersionURL{VersionURL: pkginfo.VersionURL{
			URL:           srv.URL + "/stable",
			LinuxX64URL:   srv.URL + "/grok-$version-x64",
			LinuxArm64URL: srv.URL + "/grok-$version-arm64",
		}},
		Installer: RawBin{name: "grok"},
		name:      "grok",
	}

	dir := t.TempDir()
	require.NoError(t, p.Install(dir, "1.0.5"))

	// the download lands at the dir root, executable
	//nolint:gosec // test fixture path
	body, err := os.ReadFile(filepath.Join(dir, "grok"))
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\n", string(body))
	info, err := os.Stat(filepath.Join(dir, "grok"))
	require.NoError(t, err)
	assert.Equal(t, execFileMode, info.Mode().Perm())

	// the arch template was hit with the version substituted
	want := "/grok-1.0.5-x64"
	if runtime.GOARCH == "arm64" {
		want = "/grok-1.0.5-arm64"
	}
	assert.Equal(t, []string{want}, hits)

	assert.Equal(t, "grok", p.RelBin())
}

func TestFilePkger_Latest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("1.2.3"))
	}))
	defer srv.Close()

	p := FilePkger{
		Downloader: VersionURL{VersionURL: pkginfo.VersionURL{URL: srv.URL}},
		Installer:  RawBin{name: "grok"},
		name:       "grok",
	}
	v, err := p.Latest()
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", v)
}
