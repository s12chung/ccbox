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

		resp, err := GetOK(t.Context(), nil, srv.URL, nil)
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

		_, err := GetOK(t.Context(), nil, srv.URL, nil) //nolint:bodyclose // the error path closes it
		assert.ErrorContains(t, err, "404 Not Found")
	})

	t.Run("transport error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
		srv.Close()

		_, err := GetOK(t.Context(), nil, srv.URL, nil) //nolint:bodyclose // the error path closes it
		assert.Error(t, err)
	})

	t.Run("headers go out with the request", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte("ok"))
		}))
		defer srv.Close()

		headers := http.Header{"Authorization": []string{"Bearer stub"}}
		resp, err := GetOK(t.Context(), nil, srv.URL, headers)
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck // failing is ok
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

	body, err := Body(t.Context(), nil, srv.URL, nil)
	require.NoError(t, err)
	assert.Equal(t, "1.0.5", string(body))

	_, err = Body(t.Context(), nil, srv.URL+"/missing", nil)
	assert.ErrorContains(t, err, "404 Not Found")
}
