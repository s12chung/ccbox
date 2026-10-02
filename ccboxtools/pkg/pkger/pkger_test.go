package pkger

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
)

func TestForPkgInfo(t *testing.T) {
	npm, err := ForPkgInfo(pkginfo.PkgInfo{Name: "claude", Npm: &pkginfo.Npm{Package: "a"}})
	require.NoError(t, err)
	assert.IsType(t, Npm{}, npm)
	assert.Equal(t, "claude", npm.Name())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("1.0.5"))
	}))
	defer srv.Close()

	for _, tt := range []struct {
		name          string
		vu            pkginfo.VersionURL
		wantVersioner Versioner
	}{
		{
			"template",
			pkginfo.VersionURL{URL: srv.URL, DownloadTemplate: &pkginfo.DownloadTemplate{X64URL: srv.URL, Arm64URL: srv.URL}},
			TextVersioner{},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, err := ForPkgInfo(pkginfo.PkgInfo{Name: "grok", VersionURL: &tt.vu})
			require.NoError(t, err)
			assert.Equal(t, "grok", p.Name())

			filePkger, ok := p.(FilePkger)
			require.True(t, ok)
			u, ok := filePkger.Downloader.(VersionURL)
			require.True(t, ok)
			assert.IsType(t, tt.wantVersioner, u.versioner)
			assert.IsType(t, RawBin{}, filePkger.Installer)
		})
	}

	t.Run("no download_template", func(t *testing.T) {
		_, err := ForPkgInfo(pkginfo.PkgInfo{Name: "grok", VersionURL: &pkginfo.VersionURL{
			URL: "https://x",
		}})
		assert.ErrorContains(t, err, "no download_template")
	})
}

func TestPkgDir(t *testing.T) {
	npm, err := ForPkgInfo(pkginfo.PkgInfo{Name: "claude", Npm: &pkginfo.Npm{Package: "a"}})
	require.NoError(t, err)
	d := PkgDir{Pkger: npm, Root: "/opt/ccbox/clis"}

	assert.Equal(t, "/opt/ccbox/clis/claude", d.Dir())
	assert.Equal(t, "/opt/ccbox/clis/claude/1.2.3", d.VersionDir("1.2.3"))
	assert.Equal(t, "/opt/ccbox/clis/claude/.tmp-1.2.3", d.TmpDir("1.2.3"))
	assert.Equal(t, "/opt/ccbox/clis/claude/current", d.Current())
	assert.Equal(t, "/opt/ccbox/clis/bin/claude", d.BinLink())
	assert.Equal(t, "../claude/current/bin/claude", d.BinLinkTarget())
}
