package pkger

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
)

func TestLatestAt(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"version": "1.2.3"}`))
		}))
		defer srv.Close()

		v, err := latestAt(srv.URL, "@scope/pkg")
		require.NoError(t, err)
		assert.Equal(t, "1.2.3", v)
	})

	for name, body := range map[string]string{
		"http error": `not found`,
		"no version": `{}`,
		"bad json":   `<html></html>`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()

			_, err := latestAt(srv.URL, "@scope/pkg")
			assert.Error(t, err)
		})
	}
}

func TestNpm_Install(t *testing.T) {
	var got []string
	orig := runCmd
	runCmd = func(_ string, args ...string) error { got = args; return nil }
	defer func() { runCmd = orig }()

	p := Npm{Npm: pkginfo.Npm{Package: "@scope/pkg"}, name: "pkg"}
	dir := t.TempDir()
	require.NoError(t, p.Install(dir, "1.2.3"))

	assert.Equal(t, []string{
		"install", "-g", "--prefix", dir,
		"--ignore-scripts=false", "--allow-scripts=@scope/pkg",
		"@scope/pkg@1.2.3",
	}, got)
	assert.Equal(t, "bin/pkg", p.RelBin())
}
