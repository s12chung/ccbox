package entrypoint

import (
	"errors"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/identity"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

func TestMain(m *testing.M) {
	os.Exit(testutil.MainHome(m, nil))
}

func TestExec_EmptyArgv(t *testing.T) {
	err := Exec(t.Context(), nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no command to exec")
}

func TestExec_RefusesUndroppedSession(t *testing.T) {
	// a session that fails its arrival proof must be refused before anything runs
	t.Setenv("http_proxy", "") // the no-proxy landing
	verifyAndResetDrop = func() error { return errors.New("refused") }
	t.Cleanup(func() { verifyAndResetDrop = identity.VerifyAndResetDrop })

	err := Exec(t.Context(), []string{"id"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must arrive as ccbox with no caps and no_new_privs")
}

func TestExec_RefusesProxiedBoot(t *testing.T) {
	// the drop's proof alone is unwalled on an egress wall boot: the netns proof is
	// `proxy exec`'s, so even a dropped session must be refused
	t.Setenv("http_proxy", "http://"+pkginfo.ProxyAddr+":"+strconv.Itoa(pkginfo.ProxyPort))
	verifyAndResetDrop = func() error { return nil }
	t.Cleanup(func() { verifyAndResetDrop = identity.VerifyAndResetDrop })

	err := Exec(t.Context(), []string{"id"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "proxy exec")
}

func TestExecArgv(t *testing.T) {
	t.Setenv(pkginfo.EnvVar, "") // nothing to hold — a devbox's own env would otherwise
	tests := []struct {
		name    string
		argv    []string
		wantErr string
	}{
		{"empty", nil, "no command to exec"},
		{"missing binary", []string{"ccbox-definitely-not-a-binary"}, "exec ccbox-definitely-not-a-binary"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ExecArgv(tt.argv)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestHoldForExec(t *testing.T) {
	t.Setenv(pkginfo.EnvVar, "")
	require.NoError(t, holdForExec(), "a landing whose env names no cli has no version to hold")

	t.Setenv(pkginfo.EnvVar, `{"name":`)
	err := holdForExec()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "pkginfo: parse")
}

func TestPortBindingsAndGateway(t *testing.T) {
	tests := []struct {
		name         string
		bound        []string
		gateway      []string
		serveVNC     bool
		wantBindings []uint16
		wantGateway  []uint16
		wantErr      bool
	}{
		{"no ports, no vnc", nil, nil, false, nil, nil, false},
		{"vnc only", nil, nil, true, []uint16{pkginfo.VNCPort}, nil, false},
		{"bound only", []string{"3000", "7200"}, nil, false, []uint16{3000, 7200}, nil, false},
		{"bound and vnc", []string{"3000", "7200"}, nil, true, []uint16{3000, 7200, pkginfo.VNCPort}, nil, false},
		{"bound vnc port", []string{"3000", "5900"}, nil, true, []uint16{3000, 5900}, nil, false},
		{"gateway ports", nil, []string{"5432"}, false, nil, []uint16{5432}, false},
		{"bound and gateway", []string{"3000"}, []string{"5432", "6379"}, false, []uint16{3000}, []uint16{5432, 6379}, false},
		{"garbage", []string{"3000", "http"}, nil, false, nil, nil, true},
		{"garbage gateway", nil, []string{"http"}, false, nil, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := pkginfo.RunConfig{CLIVersion: pkginfo.LatestVersion, PortBindingsTCP: tt.bound, HostGatewayTCP: tt.gateway}

			portBindings, gatewayPorts, err := portBindingsAndGateway(config, tt.serveVNC)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantBindings, portBindings)
			assert.Equal(t, tt.wantGateway, gatewayPorts)
		})
	}
}
