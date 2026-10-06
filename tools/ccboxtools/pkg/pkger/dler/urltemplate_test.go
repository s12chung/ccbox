package dler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

func TestURLTemplate_Latest(t *testing.T) {
	t.Run("text", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("  1.0.5\n"))
		}))
		defer srv.Close()

		u := NewURLTemplate(srv.URL, pkginfo.DownloadTemplate{X64URL: srv.URL, Arm64URL: srv.URL})

		v, err := u.Latest()
		require.NoError(t, err)
		assert.Equal(t, "1.0.5", v)
	})

	for _, body := range []string{"", "not found", "<html>502</html>", "1.0.5\n1.0.6"} {
		t.Run("rejects "+body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()

			u := NewURLTemplate(srv.URL, pkginfo.DownloadTemplate{X64URL: srv.URL, Arm64URL: srv.URL})

			_, err := u.Latest()
			assert.ErrorContains(t, err, "is not a version")
		})
	}

	t.Run("http error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer srv.Close()

		u := NewURLTemplate(srv.URL, pkginfo.DownloadTemplate{X64URL: srv.URL, Arm64URL: srv.URL})

		_, err := u.Latest()
		assert.ErrorContains(t, err, "502 Bad Gateway")
	})
}

func TestURLTemplate_Download(t *testing.T) {
	t.Run("text", func(t *testing.T) {
		var hits []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits = append(hits, r.URL.Path)
			if r.URL.Path == "/stable" {
				_, _ = w.Write([]byte("1.0.5"))
				return
			}
			_, _ = w.Write([]byte("#!/bin/sh\n"))
		}))
		defer srv.Close()

		u := NewURLTemplate(srv.URL+"/stable", pkginfo.DownloadTemplate{
			X64URL:   srv.URL + "/grok-$version-x64",
			Arm64URL: srv.URL + "/grok-$version-arm64",
		})

		exe, err := u.Download("1.0.5")
		require.NoError(t, err)
		defer func() { assert.NoError(t, exe.Close()) }()

		body, err := io.ReadAll(exe)
		require.NoError(t, err)
		assert.Equal(t, "#!/bin/sh\n", string(body))

		// the arch template was hit with the version substituted, no hash lookup
		want := []string{"/grok-1.0.5-x64"}
		if runtime.GOARCH == "arm64" {
			want = []string{"/grok-1.0.5-arm64"}
		}
		assert.Equal(t, want, hits)
	})

	t.Run("non-200", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/stable" {
				_, _ = w.Write([]byte("1.0.5"))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		u := NewURLTemplate(srv.URL+"/stable", pkginfo.DownloadTemplate{X64URL: srv.URL + "/exe", Arm64URL: srv.URL + "/exe"})

		_, err := u.Download("1.0.5")
		assert.ErrorContains(t, err, "404 Not Found")
	})

	t.Run("transport error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
		u := NewURLTemplate(srv.URL, pkginfo.DownloadTemplate{X64URL: srv.URL, Arm64URL: srv.URL})
		srv.Close()

		_, err := u.Download("1.0.5")
		assert.Error(t, err)
	})
}
