package entrypoint

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
			ok, err := reachable(directClient(), tt.url)

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

func TestProbeAllowed(t *testing.T) {
	ok := okServer(t).URL

	tests := []struct {
		name    string
		url     string
		wantErr string
	}{
		{"ok", ok, ""},
		{"down", downServer(t), "allowed host unreachable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := probeAllowed(directClient(), tt.url)

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestProbeBlocked(t *testing.T) {
	ok := okServer(t).URL

	tests := []struct {
		name    string
		url     string
		wantErr string
	}{
		{"rejected", downServer(t), ""}, // failing here is the success case
		{"reachable", ok, "blocked host reachable through proxy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := probeBlocked(directClient(), tt.url)

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestProbeDirect(t *testing.T) {
	ok := okServer(t).URL

	tests := []struct {
		name    string
		url     string
		wantErr string
	}{
		{"no route", downServer(t), ""},
		{"reachable", ok, "direct egress works"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := probeDirect(directClient(), tt.url)

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
