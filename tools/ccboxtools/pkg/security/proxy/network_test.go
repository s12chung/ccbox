package proxy

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

func TestBuildNetns(t *testing.T) {
	calls := recordIP(t, func([]string) error { return nil })
	ipt := recordIPTables(t, func([]string) error { return nil })
	pins := recordPins(t, func(string) error { return nil })
	stubSysctl(t, "", "", "")

	require.NoError(t, buildNetns(t.Context(), nil))

	require.Len(t, *calls, 12)
	assert.Equal(t, []string{"netns", "add", netnsName}, (*calls)[0])
	assert.Equal(t, []string{"addr", "add", pkginfo.ProxyAddr + "/30", "dev", parentVeth}, (*calls)[2], "our end completes before the hand-across")
	assert.Equal(t, []string{"link", "set", childVeth, "netns", netnsName}, (*calls)[4], "the hand-across comes after our end")
	for _, call := range (*calls)[5:11] {
		assert.Equal(t, []string{"-n", netnsName}, call[:2], "child-side config names the pinned netns")
	}
	assert.Equal(t, []string{"-6", "addr", "flush", "dev", "lo"}, (*calls)[6][2:])
	assert.Equal(t, []string{"link", "set", childVeth, "addrgenmode", "none"}, (*calls)[7][2:], "the child end never grows a v6 address")
	assert.Equal(t, []string{"addr", "add", childIP + "/30", "dev", childVeth}, (*calls)[8][2:], "the child end is addressed before it's raised")
	assert.Equal(t, []string{"route", "replace", "default", "via", pkginfo.ProxyAddr}, (*calls)[10][2:], "the exit sign comes once the road exists")
	assert.Equal(t, []string{"netns", "exec", netnsName, "sh", "-c", pinChild}, (*calls)[11], "the child pin is the build's last `ip` row")
	require.Len(t, *ipt, 2)
	assert.Equal(t, []string{"-P", "FORWARD", "DROP"}, (*ipt)[0], "the drop policy stands first — a partial apply forwards nothing")
	require.Len(t, *pins, 1)
	assert.Equal(t, pinParent, (*pins)[0], "the parent pin runs last — its veth exists and forwarding starts whitelisted")
}

func TestBuildNetns_OpensChildPorts(t *testing.T) {
	ipt := recordIPTables(t, func([]string) error { return nil })
	recordIP(t, func([]string) error { return nil })
	recordPins(t, func(string) error { return nil })
	stubSysctl(t, "", "", "")

	require.NoError(t, buildNetns(t.Context(), []uint16{5900}))

	require.Len(t, *ipt, 4)
	accept := []string{"-A", "FORWARD", "-i", "eth0", "-o", "proxy-parent", "-d", "169.254.100.2", "-p", "tcp", "--dport", "5900", "-j", "ACCEPT"}
	dnat := []string{
		"-t", "nat", "-A", "PREROUTING", "-i", "eth0", "-p", "tcp", "--dport", "5900",
		"-j", "DNAT", "--to-destination", "169.254.100.2:5900",
	}
	assert.Equal(t, accept, (*ipt)[2])
	assert.Equal(t, dnat, (*ipt)[3])
}

func TestBuildNetns_StopsAtFailure(t *testing.T) {
	calls := recordIP(t, func(args []string) error {
		if args[0] == "addr" {
			return errors.New("boom")
		}
		return nil
	})
	recordPins(t, func(string) error { return nil })
	stubSysctl(t, "", "", "")

	err := buildNetns(t.Context(), nil)

	require.Error(t, err)
	require.ErrorContains(t, err, "egress wall")
	assert.Len(t, *calls, 3, "the failed step's successors stay skipped")
}

func TestBuildNetns_FailsClosedOnIPTables(t *testing.T) {
	calls := recordIP(t, func([]string) error { return nil })
	recordIPTables(t, func([]string) error { return errors.New("boom") })
	pins := recordPins(t, func(string) error { return nil })

	err := buildNetns(t.Context(), nil)

	require.Error(t, err)
	require.ErrorContains(t, err, "egress wall")
	assert.Len(t, *calls, 12, "the whole cable builds before the table applies")
	assert.Empty(t, *pins, "the parent pin waits for the drop policy — forwarding never turns on unwhitelisted")
}

func TestBuildNetns_FailsClosedOnPin(t *testing.T) {
	calls := recordIP(t, func([]string) error { return nil })
	ipt := recordIPTables(t, func([]string) error { return nil })
	recordPins(t, func(string) error { return errors.New("boom") })

	err := buildNetns(t.Context(), nil)

	require.Error(t, err)
	require.ErrorContains(t, err, "egress wall")
	assert.Len(t, *calls, 12, "the parent pin runs after the whole build")
	assert.NotEmpty(t, *ipt, "the whitelist applies before the pin — the drop policy stands before forwarding turns on")
}

func TestBuildNetns_FailsClosedOnForwarding(t *testing.T) {
	recordIP(t, func([]string) error { return nil })
	recordIPTables(t, func([]string) error { return nil })
	recordPins(t, func(string) error { return nil })
	stubSysctl(t, "parent", ipForwardPath, "0\n")

	err := buildNetns(t.Context(), nil)

	require.Error(t, err)
	require.ErrorContains(t, err, "ip_forward=0, want 1")
}

func TestBuildNetns_FailsClosedOnRPFilter(t *testing.T) {
	recordIP(t, func([]string) error { return nil })
	recordIPTables(t, func([]string) error { return nil })
	recordPins(t, func(string) error { return nil })
	stubSysctl(t, "parent", parentRPFilterPath, "0\n")

	err := buildNetns(t.Context(), nil)

	require.Error(t, err)
	require.ErrorContains(t, err, "rp_filter=0, want 1")
}

func TestEgressConsts(t *testing.T) {
	assert.Equal(t, "/run/netns/"+netnsName, netnsPath)
	// the /30's two hosts, parent first: the child's only route out is its veth's peer
	assert.Equal(t, "169.254.100.1", pkginfo.ProxyAddr)
	assert.Equal(t, "169.254.100.2", childIP)
	assert.Equal(t, "eth0", parentIface)
}

func TestIPTablesRows(t *testing.T) {
	base := [][]string{
		{"-P", "FORWARD", "DROP"},
		{"-A", "FORWARD", "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT"},
	}
	opening := func(port string) [][]string {
		return [][]string{
			{"-A", "FORWARD", "-i", "eth0", "-o", "proxy-parent", "-d", "169.254.100.2", "-p", "tcp", "--dport", port, "-j", "ACCEPT"},
			{"-t", "nat", "-A", "PREROUTING", "-i", "eth0", "-p", "tcp", "--dport", port, "-j", "DNAT", "--to-destination", "169.254.100.2:" + port},
		}
	}
	tests := []struct {
		name       string
		childPorts []uint16
		want       [][]string
	}{
		{"no child-open ports", nil, base},
		{"one port", []uint16{5900}, slices.Concat(base, opening("5900"))},
		{"ports stay paired in order", []uint16{5900, 3389}, slices.Concat(base, opening("5900"), opening("3389"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ipTablesRows(tt.childPorts))
		})
	}
}

func TestRequirePins(t *testing.T) {
	tests := []struct {
		name    string
		side    string
		path    string
		answer  string
		wantErr string
	}{
		{"pinned", "", "", "", ""},
		{"parent forwards off", "parent", ipForwardPath, "0\n", "the parent netns's net.ipv4.ip_forward=0, want 1"},
		{"child forwards", "child", ipForwardPath, "1\n", "the child netns's net.ipv4.ip_forward=1, want 0"},
		{"parent loose", "parent", parentRPFilterPath, "0\n", "the parent veth's net.ipv4.conf.rp_filter=0, want 1"},
		{"child loose", "child", childRPFilterPath, "0\n", "the child veth's net.ipv4.conf.rp_filter=0, want 1"},
		{"parent garbage", "parent", parentRPFilterPath, "no\n", "net.ipv4.conf.rp_filter=no, want 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubSysctl(t, tt.side, tt.path, tt.answer)
			err := requirePins(t.Context())

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
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

// recordIPTables stubs buildNetns's `iptables` with a recorder, restored at cleanup.
func recordIPTables(t *testing.T, fail func(args []string) error) *[][]string {
	t.Helper()
	calls := &[][]string{}
	orig := runIPTables
	runIPTables = func(_ context.Context, args ...string) error {
		*calls = append(*calls, args)
		return fail(args)
	}
	t.Cleanup(func() { runIPTables = orig })
	return calls
}

// recordPins stubs buildNetns's pin execs with a recorder, restored at cleanup.
func recordPins(t *testing.T, fail func(script string) error) *[]string {
	t.Helper()
	calls := &[]string{}
	orig := runPin
	runPin = func(_ context.Context, script string) error {
		*calls = append(*calls, script)
		return fail(script)
	}
	t.Cleanup(func() { runPin = orig })
	return calls
}

// stubSysctl aims the proof's reads at the pins' wants — save the given side
// and path, which answer with the given answer. Empty side and path: all hold.
func stubSysctl(t *testing.T, side, path, answer string) {
	t.Helper()
	reply := func(askedSide, askedPath string) ([]byte, error) {
		for _, pin := range pins {
			if askedSide != pin.side || askedPath != pin.path {
				continue
			}
			if askedSide == side && askedPath == path {
				return []byte(answer), nil
			}
			return []byte(pin.want + "\n"), nil
		}
		return nil, fmt.Errorf("no such pin: side %s, path %s", askedSide, askedPath)
	}
	origParent, origChild := readParentSysctl, readChildSysctl
	readParentSysctl = func(_ context.Context, askedPath string) ([]byte, error) {
		return reply("parent", askedPath)
	}
	readChildSysctl = func(_ context.Context, askedPath string) ([]byte, error) {
		return reply("child", askedPath)
	}
	t.Cleanup(func() { readParentSysctl, readChildSysctl = origParent, origChild })
}
