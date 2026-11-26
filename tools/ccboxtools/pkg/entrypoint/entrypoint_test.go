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
		body         string
		gatewayBody  string
		serveVNC     bool
		wantBindings []uint16
		wantGateway  []uint16
		wantErr      bool
	}{
		{"no env, no vnc", "", "", false, nil, nil, false},
		{"vnc only", "", "", true, []uint16{pkginfo.VNCPort}, nil, false},
		{"bound only", "3000,7200", "", false, []uint16{3000, 7200}, nil, false},
		{"bound and vnc", "3000,7200", "", true, []uint16{3000, 7200, pkginfo.VNCPort}, nil, false},
		{"bound vnc port", "3000,5900", "", true, []uint16{3000, 5900}, nil, false},
		{"gateway ports", "", "5432", false, nil, []uint16{5432}, false},
		{"bound and gateway", "3000", "5432,6379", false, []uint16{3000}, []uint16{5432, 6379}, false},
		{"garbage", "3000,http", "", false, nil, nil, true},
		{"garbage gateway", "", "http", false, nil, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(pkginfo.PortBindingsTCPEnvVar, tt.body)
			t.Setenv(pkginfo.HostGatewayTCPEnvVar, tt.gatewayBody)

			portBindings, gatewayPorts, err := portBindingsAndGateway(tt.serveVNC)
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
