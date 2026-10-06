package pkger

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkger/artifact"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkger/dler"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkger/npm"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

func TestForPkgInfo(t *testing.T) {
	p, err := ForPkgInfo(pkginfo.PkgInfo{Name: "claude", Npm: &pkginfo.Npm{Package: "a"}})
	require.NoError(t, err)
	assert.IsType(t, npm.Npm{}, p)
	assert.Equal(t, "claude", p.Name())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("1.0.5"))
	}))
	defer srv.Close()

	t.Run("version url", func(t *testing.T) {
		p, err := ForPkgInfo(pkginfo.PkgInfo{Name: "grok", ReleaseURL: &pkginfo.ReleaseURL{
			URL:              srv.URL,
			DownloadTemplate: &pkginfo.DownloadTemplate{X64URL: srv.URL, Arm64URL: srv.URL},
		}})
		require.NoError(t, err)
		assert.Equal(t, "grok", p.Name())

		filePkger, ok := p.(FilePkger)
		require.True(t, ok)
		assert.IsType(t, dler.URLTemplate{}, filePkger.Downloader)
		assert.IsType(t, artifact.RawBin{}, filePkger.Installer)
		assert.Equal(t, "grok", filePkger.RelBin())
	})

	t.Run("manifest deb", func(t *testing.T) {
		p, err := ForPkgInfo(pkginfo.PkgInfo{
			Name: "zcode",
			ReleaseURL: &pkginfo.ReleaseURL{
				URL:      srv.URL + "/manifest",
				JQSchema: &pkginfo.JQSchema{Format: "yaml", Version: ".version", DownloadURL: ".url"},
				Artifact: &pkginfo.Artifact{Type: "deb", RelBin: "opt/ZCode/zcode"},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "zcode", p.Name())

		filePkger, ok := p.(FilePkger)
		require.True(t, ok)
		assert.IsType(t, dler.JQ{}, filePkger.Downloader)
		assert.IsType(t, artifact.Deb{}, filePkger.Installer)
		assert.Equal(t, "opt/ZCode/zcode", filePkger.RelBin())
	})

	t.Run("no download source", func(t *testing.T) {
		_, err := ForPkgInfo(pkginfo.PkgInfo{Name: "grok", ReleaseURL: &pkginfo.ReleaseURL{
			URL: "https://x",
		}})
		assert.ErrorContains(t, err, "no download_template or jq_schema download_url")
	})
}

func TestPkgerize_ConfigError(t *testing.T) {
	for _, tt := range []struct {
		name    string
		rel     pkginfo.ReleaseURL
		wantErr string
	}{
		{
			"no download source",
			pkginfo.ReleaseURL{URL: "https://x"},
			"no download_template or jq_schema download_url",
		},
		{
			"download_url missing",
			pkginfo.ReleaseURL{URL: "https://x", JQSchema: &pkginfo.JQSchema{Format: "yaml", Version: ".version"}},
			"no download_template or jq_schema download_url",
		},
		{
			"unknown jq format",
			pkginfo.ReleaseURL{
				URL:      "https://x",
				JQSchema: &pkginfo.JQSchema{Format: "xml", Version: ".version", DownloadURL: ".url"},
			},
			`unknown format "xml"`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pkgerize(tt.rel)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestPkgDir(t *testing.T) {
	p, err := ForPkgInfo(pkginfo.PkgInfo{Name: "claude", Npm: &pkginfo.Npm{Package: "a"}})
	require.NoError(t, err)
	d := PkgDir{Pkger: p, Root: "/opt/ccbox/clis"}

	assert.Equal(t, "/opt/ccbox/clis/claude", d.Dir())
	assert.Equal(t, "/opt/ccbox/clis/claude/1.2.3", d.VersionDir("1.2.3"))
	assert.Equal(t, "/opt/ccbox/clis/claude/.tmp-1.2.3", d.TmpDir("1.2.3"))
	assert.Equal(t, "/opt/ccbox/clis/claude/current", d.Current())
	assert.Equal(t, "/opt/ccbox/clis/bin/claude", d.BinLink())
	assert.Equal(t, "../claude/current/bin/claude", d.BinLinkTarget())
}
