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
