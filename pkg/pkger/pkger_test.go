package pkger

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNpmArg(t *testing.T) {
	assert.Equal(t, "npm:@scope/pkg", Npm{Package: "@scope/pkg"}.Arg())
}

func TestNpmLatest(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Write([]byte(`{"version": "1.2.3"}`))
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
				w.Write([]byte(body))
			}))
			defer srv.Close()

			_, err := latestAt(srv.URL, "@scope/pkg")
			assert.Error(t, err)
		})
	}
}
