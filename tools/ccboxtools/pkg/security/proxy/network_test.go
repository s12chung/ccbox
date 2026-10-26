package proxy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

func TestBuildNetns(t *testing.T) {
	calls := recordIP(t, func([]string) error { return nil })
	stubForwarding(t, "0\n")

	require.NoError(t, buildNetns(t.Context()))

	require.Len(t, *calls, 11)
	assert.Equal(t, []string{"netns", "add", netnsName}, (*calls)[0])
	assert.Equal(t, []string{"addr", "add", pkginfo.ProxyAddr + "/30", "dev", parentVeth}, (*calls)[2], "our end completes before the hand-across")
	assert.Equal(t, []string{"link", "set", childVeth, "netns", netnsName}, (*calls)[4], "the hand-across comes after our end")
	for _, call := range (*calls)[5:9] {
		assert.Equal(t, []string{"-n", netnsName}, call[:2], "child-side config names the pinned netns")
	}
	assert.Equal(t, []string{"addr", "add", childIP + "/30", "dev", childVeth}, (*calls)[6][2:], "the child end is addressed before it's raised")
	assert.Equal(t, []string{"route", "replace", "default", "via", pkginfo.ProxyAddr}, (*calls)[8][2:])
	assert.Equal(t, []string{"netns", "exec", netnsName, "sysctl", "-qw", "net.ipv6.conf.default.disable_ipv6=1"}, (*calls)[len(*calls)-1])
}

func TestBuildNetns_StopsAtFailure(t *testing.T) {
	calls := recordIP(t, func(args []string) error {
		if args[0] == "addr" {
			return errors.New("boom")
		}
		return nil
	})
	stubForwarding(t, "0\n")

	err := buildNetns(t.Context())

	require.Error(t, err)
	require.ErrorContains(t, err, "egress wall")
	assert.Len(t, *calls, 3, "the failed step's successors stay skipped")
}

func TestBuildNetns_FailsClosedOnForwarding(t *testing.T) {
	recordIP(t, func([]string) error { return nil })
	stubForwarding(t, "1\n")

	err := buildNetns(t.Context())

	require.Error(t, err)
	require.ErrorContains(t, err, "ip_forward=1, want 0")
}

// recordIP stubs buildNetns's `ip` with a recorder, restored at cleanup.
func recordIP(t *testing.T, fail func(args []string) error) *[][]string {
	t.Helper()
	calls := &[][]string{}
	orig := runIP
	runIP = func(_ context.Context, args ...string) error {
		*calls = append(*calls, args)
		return fail(args)
	}
	t.Cleanup(func() { runIP = orig })
	return calls
}

// stubForwarding aims the proof's read at a temp file carrying body.
func stubForwarding(t *testing.T, body string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ip_forward")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	orig := ipForwardPath
	ipForwardPath = path
	t.Cleanup(func() { ipForwardPath = orig })
}

func TestEgressConsts(t *testing.T) {
	assert.Equal(t, "/run/netns/"+netnsName, netnsPath)
	// the /30's two hosts, parent first: the child's only route out is its veth's peer
	assert.Equal(t, "169.254.100.1", pkginfo.ProxyAddr)
	assert.Equal(t, "169.254.100.2", childIP)
}

func TestRequireNoForwarding(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{"off", "0\n", ""},
		{"on", "1\n", "net.ipv4.ip_forward=1, want 0"},
		{"unreadable garbage", "yes\n", "net.ipv4.ip_forward=yes, want 0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubForwarding(t, tt.body)
			err := requireNoForwarding()

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestNSIsolatedCmd(t *testing.T) {
	cmd, err := NSIsolatedCmd(t.Context(), "exec", "--", "bash")
	require.NoError(t, err)

	assert.Contains(t, cmd.Path, nsenterBin) // LookPath resolves at construction
	self, err := os.Executable()
	require.NoError(t, err)
	assert.Equal(t, []string{"nsenter", "--net=" + netnsPath, "--", self, "exec", "--", "bash"}, cmd.Args)
	assert.Same(t, os.Stdin, cmd.Stdin)
	assert.Same(t, os.Stdout, cmd.Stdout)
	assert.Same(t, os.Stderr, cmd.Stderr)
}

func TestSameNetNS(t *testing.T) {
	// the test process sits in its own netns, not the pinned child's
	assert.False(t, SameNetNS())
}

func TestRequireNetNS(t *testing.T) {
	// the test process sits in its own netns, not the pinned child's
	err := RequireNetNS()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in the egress wall's netns")
}
