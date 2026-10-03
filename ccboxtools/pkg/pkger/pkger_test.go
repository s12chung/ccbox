package pkger

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkger/artifact"
	"github.com/s12chung/ccbox/ccboxtools/pkg/pkger/dler"
	"github.com/s12chung/ccbox/ccboxtools/pkg/pkger/npm"
	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
)

func TestForPkgInfo(t *testing.T) {
	n, err := ForPkgInfo(pkginfo.PkgInfo{Name: "claude", Npm: &pkginfo.Npm{Package: "a"}})
	require.NoError(t, err)
	assert.IsType(t, npm.Npm{}, n)
	assert.Equal(t, "claude", n.Name())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("1.0.5"))
	}))
	defer srv.Close()

	t.Run("template", func(t *testing.T) {
		p, err := ForPkgInfo(pkginfo.PkgInfo{Name: "grok", ReleaseURL: &pkginfo.ReleaseURL{
			URL:              srv.URL,
			DownloadTemplate: &pkginfo.DownloadTemplate{X64URL: srv.URL, Arm64URL: srv.URL},
		}})
		require.NoError(t, err)
		assert.Equal(t, "grok", p.Name())

		filePkger, ok := p.(FilePkger)
		require.True(t, ok)
		_, ok = filePkger.Downloader.(dler.URLTemplate)
		assert.True(t, ok)
		assert.IsType(t, artifact.RawBin{}, filePkger.Installer)
	})

	t.Run("no download_template", func(t *testing.T) {
		_, err := ForPkgInfo(pkginfo.PkgInfo{Name: "grok", ReleaseURL: &pkginfo.ReleaseURL{
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
