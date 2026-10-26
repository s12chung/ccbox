package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// okServer serves HTTP 200; downServer reserves a port and closes it, so dialing
// it is connection-refused.
func okServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	return server
}

func downServer(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()
	return server.URL
}

func directClient() *http.Client { return probeClient(nil) }

func TestReachable(t *testing.T) {
	server500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server500.Close)

	tests := []struct {
		name        string
		url         string
		wantOK      bool
		errContains string
	}{
		{"ok", okServer(t).URL, true, ""},
		{"http 500", server500.URL, false, "http 500"},
		{"down", downServer(t), false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := reachable(t.Context(), directClient(), tt.url)

			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			if tt.errContains != "" {
				assert.Contains(t, err.Error(), tt.errContains)
			}
		})
	}
}

func TestProbeProxy(t *testing.T) {
	ok := okServer(t).URL
	down := downServer(t)

	tests := []struct {
		name    string
		allowed string
		blocked string
		direct  string
		wantErr string
	}{
		{"pass", ok, down, down, ""},
		{"allowed unreachable", down, down, down, "allowed host unreachable"},
		{"block not enforced", ok, ok, down, "blocked host reachable through proxy"},
		{"direct egress works", ok, down, ok, "direct egress works"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// no proxy in the middle: the clients dial straight, so an ok server
			// stands in for "reachable through the proxy" and a down one for
			// "rejected by it"
			t.Setenv("http_proxy", "")
			t.Setenv(allowedURLVar, tt.allowed)
			t.Setenv(blockedURLVar, tt.blocked)
			t.Setenv(directURLVar, tt.direct)

			err := probeProxy(t.Context())

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
