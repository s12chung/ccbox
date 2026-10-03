package httputil

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOK(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("1.0.5"))
		}))
		defer srv.Close()

		resp, err := GetOK(nil, srv.URL)
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck // failing is ok

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, "1.0.5", string(body))
	})

	t.Run("non-200", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		_, err := GetOK(nil, srv.URL) //nolint:bodyclose // the error path closes it
		assert.ErrorContains(t, err, "404 Not Found")
	})

	t.Run("transport error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
		srv.Close()

		_, err := GetOK(nil, srv.URL) //nolint:bodyclose // the error path closes it
		assert.Error(t, err)
	})
}

func TestBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte("1.0.5"))
	}))
	defer srv.Close()

	body, err := Body(nil, srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "1.0.5", string(body))

	_, err = Body(nil, srv.URL+"/missing")
	assert.ErrorContains(t, err, "404 Not Found")
}
