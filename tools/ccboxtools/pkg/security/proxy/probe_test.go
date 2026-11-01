package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
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

func directClient() *http.Client { return probeClient(nil, directProbeMax) }

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

func directFrom() func(*http.Request) (*url.URL, error) {
	return http.ProxyURL(nil)
}

func TestProbeProxy(t *testing.T) {
	ok := okServer(t).URL
	down := downServer(t)

	// the share pin is TestVerifyShareRO's: aim the stubs so this test stays on
	// the policy probes
	origShare := probeShareRO
	probeShareRO = func() error { return syscall.EROFS }
	t.Cleanup(func() { probeShareRO = origShare })
	origOpen := openAllow
	openAllow = func() error { return syscall.EROFS }
	t.Cleanup(func() { openAllow = origOpen })
	origProxy := proxyFrom
	proxyFrom = directFrom()
	t.Cleanup(func() { proxyFrom = origProxy })
	origAllowed, origBlocked, origDirect := allowedURL, blockedURL, directURL
	t.Cleanup(func() { allowedURL, blockedURL, directURL = origAllowed, origBlocked, origDirect })

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
			allowedURL, blockedURL, directURL = tt.allowed, tt.blocked, tt.direct

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

func TestProxyFrom_PinsTheWallProxy(t *testing.T) {
	u, err := proxyFrom(nil)

	require.NoError(t, err)
	assert.Equal(t, "http", u.Scheme)
	assert.Equal(t, fmt.Sprintf("%s:%d", pkginfo.ProxyAddr, pkginfo.ProxyPort), u.Host)
}

func TestVerifyShareRO(t *testing.T) {
	tests := []struct {
		name     string
		shareErr error
		allowErr error
		wantErr  string
	}{
		{"pinned", syscall.EROFS, syscall.EROFS, ""},
		{"share writable", nil, syscall.EROFS, pkginfo.ProxyMount + " is writable"},
		{"allow writable", syscall.EROFS, nil, pkginfo.ProxyAllowPath + " is writable"},
		{"denied another way", syscall.EACCES, syscall.EROFS, "verify share ro"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probeShareRO = func() error { return tt.shareErr }
			openAllow = func() error { return tt.allowErr }
			t.Cleanup(func() { probeShareRO = mkdirInShare })
			t.Cleanup(func() { openAllow = openAllowFile })

			err := verifyShareRO()

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
