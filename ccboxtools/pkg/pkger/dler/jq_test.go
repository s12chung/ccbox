package dler

import (
	"crypto/sha512"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
)

// zcodeManifest is the app's auto-update feed for linux-x64, served at
// https://zcode.z.ai/api/v1/releases/electron/manifest — block-scalar release
// notes included: selectors must keep reading structure, not free text.
const zcodeManifest = `version: 3.14.4
files:
    - url: https://cdn-zcode.z.ai/zcode/electron/releases/3.14.4/linux-x64/ZCode-3.14.4-linux-x64.AppImage
      sha512: eo2YOiGHOk/kprnxkiJ0LGwmcnXU0cEsaFAp2adFSod0QI9PFW+pqsELxCpsKiM8p1pEoyOB189Tao4lIqzJuQ==
      size: 204042067
    - url: https://cdn-zcode.z.ai/zcode/electron/releases/3.14.4/linux-x64/ZCode-3.14.4-linux-x64.deb
      sha512: CMaAUbHehEEGuClDJeJI30ufA0zAgzfXgzV4ES200EuGDt0yYhEpzvtjayxvE/ag9T9ThnDg1eP4o3j9HkCHKg==
      size: 162061444
    - url: https://cdn-zcode.z.ai/zcode/electron/releases/3.14.4/linux-x64/ZCode-3.14.4-linux-x64.rpm
      sha512: pPjNEcB54zt/HsVC0xnprR+XuYc6MmDr0TQDxwDsVvynm5rOWpLm/aTNRXBJ6jU9O8Cj6KxYP9wexsVuVhB30Q==
      size: 130915957
    - url: https://cdn-zcode.z.ai/zcode/electron/releases/3.14.4/linux-x64/ZCode-3.14.4-linux-x64.pkg.tar.zst
      sha512: +/2WKA4HGWdbye5La94tgZ/5kaX8FYvJlq+qt0BR+Q30nzSg2tCrm+KmNSEV0m6Nt8aTOYEmxKviFP+YBhUBIQ==
      size: 142855520
path: https://cdn-zcode.z.ai/zcode/electron/releases/3.14.4/linux-x64/ZCode-3.14.4-linux-x64.AppImage
sha512: eo2YOiGHOk/kprnxkiJ0LGwmcnXU0cEsaFAp2adFSod0QI9PFW+pqsELxCpsKiM8p1pEoyOB189Tao4lIqzJuQ==
releaseName: Release v3.14.4
releaseNotes: |
    ## 问题修复
    - 为了进一步优化免费套餐的使用体验，关闭模型请求验证码校验。
releaseNotesByLocale:
    en-US:
        title: ""
        markdown: |-
            ## Bug Fixes
            - Disabled CAPTCHA verification for model requests to further improve the free tier experience.
    zh-CN:
        title: ""
        markdown: |
            ## 问题修复
            - 为了进一步优化免费套餐的使用体验，关闭模型请求验证码校验。
releaseDate: "2026-09-29T03:01:37.416Z"
`

var zcodeSchema = pkginfo.JQSchema{
	Format:      "yaml",
	Version:     ".version",
	DownloadURL: `.files[] | select(.url | endswith(".deb")) | .url`,
	Sha512:      `.files[] | select(.url | endswith(".deb")) | .sha512`,
}

// zcodeDebSha512 is the manifest's pinned sha512 for the deb entry.
const zcodeDebSha512 = "CMaAUbHehEEGuClDJeJI30ufA0zAgzfXgzV4ES200EuGDt0yYhEpzvtjayxvE/ag9T9ThnDg1eP4o3j9HkCHKg=="

// zcodeManifestServing rewrites the manifest's cdn to the test server and, when
// content is non-nil, re-pins the deb's sha512 to content's, so the download
// through zcodeSchema verifies against it.
func zcodeManifestServing(host string, content []byte) string {
	m := strings.ReplaceAll(zcodeManifest, "https://cdn-zcode.z.ai", host)
	if content != nil {
		sum := sha512.Sum512(content)
		pin := base64.StdEncoding.EncodeToString(sum[:])
		m = strings.Replace(m, zcodeDebSha512, pin, 1)
	}
	return m
}

// newZcodeDownloader builds the manifest downloader against a TLS test server
func newZcodeDownloader(t *testing.T, content []byte, repin bool) (JQ, *httptest.Server, func() []string) {
	t.Helper()
	var hits []string
	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, ".deb") {
			_, _ = w.Write(content)
			return
		}
		pinned := content
		if !repin {
			pinned = nil
		}
		_, _ = w.Write([]byte(zcodeManifestServing(srv.URL, pinned)))
	}))
	t.Cleanup(srv.Close)

	d, err := NewJQ(srv.URL+"/linux-x64/manifest", &zcodeSchema)
	require.NoError(t, err)
	d.client = srv.Client()
	return d, srv, func() []string { return hits }
}

// zcodeDebPath is the manifest's deb url on the test server, relative to it.
func zcodeDebPath(host string) string {
	deb := regexp.MustCompile(`https://\S+\.deb`).FindString(zcodeManifest)
	return strings.TrimPrefix(strings.Replace(deb, "https://cdn-zcode.z.ai", host, 1), host)
}

func TestJQ_Latest(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"version":"1.2.3"}`))
		}))
		defer srv.Close()

		d, err := NewJQ(srv.URL, &pkginfo.JQSchema{Format: "json", Version: ".version", DownloadURL: ".url"})
		require.NoError(t, err)

		v, err := d.Latest()
		require.NoError(t, err)
		assert.Equal(t, "1.2.3", v)
	})

	tests := []struct {
		name    string
		schema  pkginfo.JQSchema
		body    string
		wantErr string
	}{
		{
			"missing key",
			pkginfo.JQSchema{Format: "yaml", Version: ".missing", DownloadURL: ".url"},
			"version: 3.14.4\n",
			`selector ".missing": result is not a string`,
		},
		{
			"multi-result",
			pkginfo.JQSchema{Format: "yaml", Version: ".files[].url", DownloadURL: ".url"},
			"files:\n  - url: a.deb\n  - url: b.rpm\n",
			"more than one result",
		},
		{
			"no result",
			pkginfo.JQSchema{Format: "yaml", Version: ".files[].url", DownloadURL: ".url"},
			"files: []\n",
			`selector ".files[].url": no result`,
		},
		{
			"bad yaml",
			pkginfo.JQSchema{Format: "yaml", Version: ".version", DownloadURL: ".url"},
			"version: [unclosed\n",
			"parse yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			d, err := NewJQ(srv.URL, &tt.schema)
			require.NoError(t, err)

			_, err = d.Latest()
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestNewJQ_UnknownFormat(t *testing.T) {
	_, err := NewJQ("https://x", &pkginfo.JQSchema{Format: "xml", Version: ".version", DownloadURL: ".url"})
	require.ErrorContains(t, err, `unknown format "xml"`)
}

func TestJQ_Download(t *testing.T) {
	content := []byte("deb-bytes")

	t.Run("happy", func(t *testing.T) {
		d, srv, hits := newZcodeDownloader(t, content, true)

		v, err := d.Latest()
		require.NoError(t, err)
		assert.Equal(t, "3.14.4", v)

		rc, err := d.Download(v)
		require.NoError(t, err)
		defer func() { assert.NoError(t, rc.Close()) }()

		body, err := io.ReadAll(rc)
		require.NoError(t, err)
		assert.Equal(t, string(content), string(body))

		// the manifest fetches — Latest, then Download's drift guard — then the
		// deb url the manifest names
		assert.Equal(t, []string{"/linux-x64/manifest", "/linux-x64/manifest", zcodeDebPath(srv.URL)}, hits())
	})

	t.Run("hash mismatch", func(t *testing.T) {
		d, _, _ := newZcodeDownloader(t, content, false)

		rc, err := d.Download("3.14.4")
		require.NoError(t, err)

		_, err = io.ReadAll(rc)
		require.ErrorContains(t, err, "sha512 mismatch")
		// the failure is the final read; Close stays clean
		assert.NoError(t, rc.Close())
	})

	t.Run("no url", func(t *testing.T) {
		d, _, _ := newZcodeDownloader(t, content, true)

		schema := zcodeSchema
		schema.DownloadURL = `.files[] | select(.url | endswith(".nope")) | .url`
		d.JQSchema = schema

		_, err := d.Download("3.14.4")
		assert.ErrorContains(t, err, "no result")
	})

	t.Run("bad url", func(t *testing.T) {
		d, _, _ := newZcodeDownloader(t, content, true)

		schema := zcodeSchema
		schema.DownloadURL = ".version" // "3.14.4" is not an https url
		d.JQSchema = schema

		_, err := d.Download("3.14.4")
		assert.ErrorContains(t, err, "DownloadURL does not match ^https://")
	})

	t.Run("bad sha512", func(t *testing.T) {
		d, _, _ := newZcodeDownloader(t, content, true)

		schema := zcodeSchema
		schema.Sha512 = ".version" // "3.14.4" is not a base64 sha512
		d.JQSchema = schema

		_, err := d.Download("3.14.4")
		assert.ErrorContains(t, err, "is not a base64 std sha512")
	})

	t.Run("no sha512 selector", func(t *testing.T) {
		// the manifest still pins the real deb's hash (repin false): the read's
		// success shows firm's optional-selector empty pass and no verification
		d, _, _ := newZcodeDownloader(t, content, false)

		schema := zcodeSchema
		schema.Sha512 = ""
		d.JQSchema = schema

		rc, err := d.Download("3.14.4")
		require.NoError(t, err)
		defer func() { assert.NoError(t, rc.Close()) }()

		body, err := io.ReadAll(rc)
		require.NoError(t, err)
		assert.Equal(t, string(content), string(body))
	})

	t.Run("no version selector", func(t *testing.T) {
		d, _, _ := newZcodeDownloader(t, content, true)

		schema := zcodeSchema
		schema.Version = "" // not firm-tolerated: the drift guard's Equal fails
		d.JQSchema = schema

		_, err := d.Download("3.14.4")
		assert.ErrorContains(t, err, "Version is not equal to 3.14.4")
	})

	t.Run("version drift", func(t *testing.T) {
		d, _, _ := newZcodeDownloader(t, content, true)

		_, err := d.Download("9.9.9")
		assert.ErrorContains(t, err, "Version is not equal to 9.9.9")
	})
}
