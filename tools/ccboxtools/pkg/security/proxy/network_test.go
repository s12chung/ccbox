package proxy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

	require.NoError(t, buildNetns(t.Context(), nil, nil))

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

func TestBuildNetns_OpensPortBindings(t *testing.T) {
	ipt := recordIPTables(t, func([]string) error { return nil })
	recordIP(t, func([]string) error { return nil })
	recordPins(t, func(string) error { return nil })
	stubSysctl(t, "", "", "")

	require.NoError(t, buildNetns(t.Context(), []uint16{5900}, nil))

	require.Len(t, *ipt, 4)
	accept := []string{"-A", "FORWARD", "-i", "eth0", "-o", "proxy-parent", "-d", "169.254.100.2", "-p", "tcp", "--dport", "5900", "-j", "ACCEPT"}
	dnat := []string{
		"-t", "nat", "-A", "PREROUTING", "-i", "eth0", "-p", "tcp", "--dport", "5900",
		"-j", "DNAT", "--to-destination", "169.254.100.2:5900",
	}
	assert.Equal(t, accept, (*ipt)[2])
	assert.Equal(t, dnat, (*ipt)[3])
}

func TestBuildNetns_OpensGatewayPorts(t *testing.T) {
	ipt := recordIPTables(t, func([]string) error { return nil })
	recordIP(t, func([]string) error { return nil })
	recordPins(t, func(string) error { return nil })
	stubSysctl(t, "", "", "")
	stubHostGatewayIP(t, "192.0.2.1")

	require.NoError(t, buildNetns(t.Context(), nil, []uint16{5432}))

	require.Len(t, *ipt, 4)
	accept := []string{"-A", "FORWARD", "-i", "proxy-parent", "-o", "eth0", "-d", "192.0.2.1", "-p", "tcp", "--dport", "5432", "-j", "ACCEPT"}
	masq := []string{"-t", "nat", "-A", "POSTROUTING", "-o", "eth0", "-d", "192.0.2.1", "-p", "tcp", "--dport", "5432", "-j", "MASQUERADE"}
	assert.Equal(t, accept, (*ipt)[2])
	assert.Equal(t, masq, (*ipt)[3])
}

func TestBuildNetns_FailsClosedOnHostGateway(t *testing.T) {
	ipt := recordIPTables(t, func([]string) error { return nil })
	recordIP(t, func([]string) error { return nil })
	pins := recordPins(t, func(string) error { return nil })
	stubHostGatewayIP(t, "")

	err := buildNetns(t.Context(), nil, []uint16{5432})

	require.Error(t, err)
	assert.Empty(t, *ipt, "a missing entry never applies a half table")
	assert.Empty(t, *pins, "forwarding never turns on unwhitelisted")
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

	err := buildNetns(t.Context(), nil, nil)

	require.Error(t, err)
	require.ErrorContains(t, err, "egress wall")
	assert.Len(t, *calls, 3, "the failed step's successors stay skipped")
}

func TestBuildNetns_FailsClosedOnIPTables(t *testing.T) {
	calls := recordIP(t, func([]string) error { return nil })
	recordIPTables(t, func([]string) error { return errors.New("boom") })
	pins := recordPins(t, func(string) error { return nil })

	err := buildNetns(t.Context(), nil, nil)

	require.Error(t, err)
	require.ErrorContains(t, err, "egress wall")
	assert.Len(t, *calls, 12, "the whole cable builds before the table applies")
	assert.Empty(t, *pins, "the parent pin waits for the drop policy — forwarding never turns on unwhitelisted")
}

func TestBuildNetns_FailsClosedOnPin(t *testing.T) {
	calls := recordIP(t, func([]string) error { return nil })
	ipt := recordIPTables(t, func([]string) error { return nil })
	recordPins(t, func(string) error { return errors.New("boom") })

	err := buildNetns(t.Context(), nil, nil)

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

	err := buildNetns(t.Context(), nil, nil)

	require.Error(t, err)
	require.ErrorContains(t, err, "ip_forward=0, want 1")
}

func TestBuildNetns_FailsClosedOnRPFilter(t *testing.T) {
	recordIP(t, func([]string) error { return nil })
	recordIPTables(t, func([]string) error { return nil })
	recordPins(t, func(string) error { return nil })
	stubSysctl(t, "parent", parentRPFilterPath, "0\n")

	err := buildNetns(t.Context(), nil, nil)

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
	exiting := func(ip, port string) [][]string {
		return [][]string{
			{"-A", "FORWARD", "-i", "proxy-parent", "-o", "eth0", "-d", ip, "-p", "tcp", "--dport", port, "-j", "ACCEPT"},
			{"-t", "nat", "-A", "POSTROUTING", "-o", "eth0", "-d", ip, "-p", "tcp", "--dport", port, "-j", "MASQUERADE"},
		}
	}
	tests := []struct {
		name         string
		portBindings []uint16
		gatewayPorts []uint16
		gwIP         string
		want         [][]string
		wantErr      string
	}{
		{"no ports", nil, nil, "", base, ""},
		{"one bound port", []uint16{5900}, nil, "", slices.Concat(base, opening("5900")), ""},
		{"bound ports stay paired in order", []uint16{5900, 3389}, nil, "", slices.Concat(base, opening("5900"), opening("3389")), ""},
		{"one gateway port", nil, []uint16{5432}, "192.0.2.1", slices.Concat(base, exiting("192.0.2.1", "5432")), ""},
		{
			"gateway ports follow the bindings",
			[]uint16{3000},
			[]uint16{5432, 6379},
			"192.0.2.1",
			slices.Concat(base, opening("3000"), exiting("192.0.2.1", "5432"), exiting("192.0.2.1", "6379")), "",
		},
		{"a missing hosts entry fails closed", nil, []uint16{5432}, "", nil, "error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubHostGatewayIP(t, tt.gwIP)

			rows, err := ipTablesRows(tt.portBindings, tt.gatewayPorts)
			if tt.wantErr != "" {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, rows)
		})
	}
}

func TestHostGatewayIP(t *testing.T) {
	hosts := func(body string) string {
		path := filepath.Join(t.TempDir(), "hosts")
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
		return path
	}
	tests := []struct {
		name    string
		path    string
		want    string
		wantErr string
	}{
		{"the entry's address", hosts("127.0.0.1 localhost\n192.0.2.1 host.docker.internal\n"), "192.0.2.1", ""},
		{"a commented entry is no entry", hosts("# 192.0.2.1 host.docker.internal\n"), "", "no " + pkginfo.HostGatewayHost},
		{"a missing file fails", filepath.Join(t.TempDir(), "absent"), "", "read"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := hostsPath
			hostsPath = tt.path
			t.Cleanup(func() { hostsPath = orig })

			ip, err := hostGatewayIP()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, ip)
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

// stubHostGatewayIP aims the host rows' /etc/hosts read at the given address —
// empty fails it, as a missing entry would.
func stubHostGatewayIP(t *testing.T, ip string) {
	t.Helper()
	orig := hostGatewayIP
	hostGatewayIP = func() (string, error) {
		if ip == "" {
			return "", errors.New("no entry")
		}
		return ip, nil
	}
	t.Cleanup(func() { hostGatewayIP = orig })
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
