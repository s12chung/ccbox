package proxy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/tinyproxy"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/supervise"
)

// The egress wall's fixed /30 — parent: tinyproxy's listen, child: the
// workload's address. Literal: the child netns resolves no names, no shared loopback.
const (
	childIP = "169.254.100.2"

	parentVeth = "proxy-parent"
	childVeth  = "proxy-child"

	// host traffic arrives here — the default bridge's iface, where pkg/docker's publish binds land
	parentIface = "eth0"

	// The sysctls' procfs paths — consts: tests stub the readers, not the paths.
	ipForwardPath      = "/proc/sys/net/ipv4/ip_forward"
	parentRPFilterPath = "/proc/sys/net/ipv4/conf/" + parentVeth + "/rp_filter"
	childRPFilterPath  = "/proc/sys/net/ipv4/conf/" + childVeth + "/rp_filter"

	// The pins — the netns sysctls are born wrong and /proc/sys is read-only,
	// so each writes through a rw proc mount, root-owned so the workload can't pre-plant it.
	pinChild = "mkdir -p /run/netns/.proc-pin && mount -t proc proc /run/netns/.proc-pin" + //nolint:dupword // fstype and source, two "proc"s
		" && echo 0 > /run/netns/.proc-pin/sys/net/ipv4/ip_forward" +
		" && echo 1 > /run/netns/.proc-pin/sys/net/ipv4/conf/" + childVeth + "/rp_filter" + // per-iface: effective is max(conf.all, conf.iface)
		" && umount /run/netns/.proc-pin && rmdir /run/netns/.proc-pin"
	pinParent = "mkdir -p /run/netns/.proc-pin && mount -t proc proc /run/netns/.proc-pin" + //nolint:dupword // fstype and source, two "proc"s
		" && echo 1 > /run/netns/.proc-pin/sys/net/ipv4/ip_forward" + // the whitelist forwards only the child-open ports into the room
		" && echo 1 > /run/netns/.proc-pin/sys/net/ipv4/conf/" + parentVeth + "/rp_filter" + // per-iface: conf/all would sweep eth0 too
		" && umount /run/netns/.proc-pin && rmdir /run/netns/.proc-pin"
)

// buildNetns's kernel touchpoints — package vars so tests aim them at stubs.
var (
	// runIP execs the build's `ip` rows — a var so tests aim it at a recorder.
	runIP = func(ctx context.Context, args ...string) error {
		//nolint:gosec // the build's fixed `ip` rows
		out, err := exec.CommandContext(ctx, "ip", args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("ip %s: %w\n%s", strings.Join(args, " "), err, out)
		}
		return nil
	}
	// runIPTables execs the build's `iptables` rows — a var so tests aim it at a recorder.
	runIPTables = func(ctx context.Context, args ...string) error {
		//nolint:gosec // the build's fixed iptables rows
		out, err := exec.CommandContext(ctx, "iptables", args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("iptables %s: %w\n%s", strings.Join(args, " "), err, out)
		}
		return nil
	}
	// runPin execs the parent-side pin script — a var so tests aim it at a recorder.
	runPin = func(ctx context.Context, script string) error {
		//nolint:gosec // the build's fixed pin scripts
		out, err := exec.CommandContext(ctx, "sh", "-c", script).CombinedOutput()
		if err != nil {
			return fmt.Errorf("sh -c %s: %w\n%s", script, err, out)
		}
		return nil
	}

	// the pins' reads: the parent's from this mount, the child's from inside
	// its netns — vars so tests aim them at stubs.
	readParentSysctl = parentSysctl
	readChildSysctl  = childSysctl
)

// CreateNetworkAndForkChild brings the egress wall up, failing closed —
// childPorts open into the child — and returns the tinyproxy supervisee for
// the caller's run loop.
func CreateNetworkAndForkChild(ctx context.Context, childPorts []uint16) (*supervise.Respawnable, error) {
	if err := buildNetns(ctx, childPorts); err != nil {
		return nil, err
	}
	tproxy := &supervise.Respawnable{
		Name: "tinyproxy", CmdToStart: tinyproxy.Cmd,
		Ready: func(ctx context.Context, cmd *exec.Cmd) error {
			if err := tinyproxy.AwaitListen(ctx, cmd); err != nil {
				return err
			}
			if err := tinyproxy.WatchAllowlist(ctx, func() int { return cmd.Process.Pid }); err != nil {
				return err
			}
			return forkProxyChild(ctx)
		},
	}
	return tproxy, nil
}

// buildNetns builds the egress wall's network: the iproute2 cable, the iptables
// whitelist, the pins — in order, so the drop policy stands before forwarding
// turns on.
func buildNetns(ctx context.Context, childPorts []uint16) error {
	for _, args := range ipArgs {
		if err := runIP(ctx, args...); err != nil {
			return fmt.Errorf("egress wall: %w", err)
		}
	}
	for _, args := range ipTablesRows(childPorts) {
		if err := runIPTables(ctx, args...); err != nil {
			return fmt.Errorf("egress wall: %w", err)
		}
	}
	if err := runPin(ctx, pinParent); err != nil {
		return fmt.Errorf("egress wall: %w", err)
	}
	return requirePins(ctx)
}

// ipArgs is the egress wall's build table: one `ip` invocation per row, run
// in order — the order is load-bearing.
//
//nolint:goconst // the rows read as the shell commands they run; named tokens would obscure them
var ipArgs = [][]string{
	{"netns", "add", netnsName}, // the named netns
	{"link", "add", parentVeth, "type", "veth", "peer", "name", childVeth}, // the cable
	{"addr", "add", pkginfo.ProxyAddr + "/30", "dev", parentVeth},          // its address
	{"link", "set", parentVeth, "up"},                                      // raised

	{"link", "set", childVeth, "netns", netnsName}, // our end complete — hand the child end across

	{"-n", netnsName, "link", "set", "lo", "up"},                        // their end, from inside: the loopback
	{"-n", netnsName, "-6", "addr", "flush", "dev", "lo"},               // no IPv6: ::1 flushed
	{"-n", netnsName, "link", "set", childVeth, "addrgenmode", "none"},  // keeps it off for good — sysctl's out (read-only /proc/sys)
	{"-n", netnsName, "addr", "add", childIP + "/30", "dev", childVeth}, // its end of the cable
	{"-n", netnsName, "link", "set", childVeth, "up"},

	{"-n", netnsName, "route", "replace", "default", "via", pkginfo.ProxyAddr}, // the room's default route — once its end is up

	{"netns", "exec", netnsName, "sh", "-c", pinChild}, // the room's forwarding off, its cable end strict
}

// ipTablesBase is the egress wall's fixed iptables rows.
//
//nolint:goconst // the rows read as the shell commands they run; named tokens would obscure them
var ipTablesBase = [][]string{
	{"-P", "FORWARD", "DROP"}, // every accept below names its flow exactly
	{"-A", "FORWARD", "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT"}, // the DNAT'd flows' replies leave
}

// ipTablesRows is the egress wall's iptables table — drop policy first, so a
// partial apply forwards nothing; no IPv6, the cable carries none.
//
//nolint:goconst // the rows read as the shell commands they run; named tokens would obscure them
func ipTablesRows(childPorts []uint16) [][]string {
	rows := make([][]string, 0, len(ipTablesBase)+2*len(childPorts))
	rows = append(rows, ipTablesBase...)
	for _, port := range childPorts {
		p := strconv.FormatUint(uint64(port), 10)
		rows = append(rows,
			[]string{"-A", "FORWARD", "-i", parentIface, "-o", parentVeth, "-d", childIP, "-p", "tcp", "--dport", p, "-j", "ACCEPT"},
			[]string{"-t", "nat", "-A", "PREROUTING", "-i", parentIface, "-p", "tcp", "--dport", p, "-j", "DNAT", "--to-destination", childIP + ":" + p},
		)
	}
	return rows
}

// pin is one sysctl one side of the cable must answer: the value the egress wall
// asserts in that side's netns.
type pin struct {
	sysctl  string
	side    string // whose netns answers the read
	kind    string // completes the error subject — the side's netns or its veth
	path    string // this mount; the child's side reads the same tree from inside its netns
	want    string
	because string
}

const (
	parentSide = "parent"
	childSide  = "child"
)

// pins is the assert table, in order: forwarding first, then the cable ends.
var pins = []pin{
	// the parent forwards only what the whitelist accepts — the child-open ports
	{
		sysctl:  "net.ipv4.ip_forward",
		side:    parentSide,
		kind:    "netns",
		path:    ipForwardPath,
		want:    "1",
		because: "the whitelist is the room's only entrance",
	},
	// the child can reach the parent's own IPs directly — that's local
	// delivery, not forwarding; ip_forward=0 doesn't stop it, and only the
	// boot's own code could ever listen there
	{
		sysctl:  "net.ipv4.ip_forward",
		side:    childSide,
		kind:    "netns",
		path:    ipForwardPath,
		want:    "0",
		because: "the child's traffic must blackhole, not forward",
	},
	{
		sysctl:  "net.ipv4.conf.rp_filter",
		side:    parentSide,
		kind:    "veth",
		path:    parentRPFilterPath,
		want:    "1",
		because: "spoofed-source replies would leave unallowlisted",
	},
	{
		sysctl:  "net.ipv4.conf.rp_filter",
		side:    childSide,
		kind:    "veth",
		path:    childRPFilterPath,
		want:    "1",
		because: "spoofed-source replies would leave unallowlisted",
	},
}

// requirePins asserts the pins hold — the child's only ways out are the
// parent-local tinyproxy and the child-open ports.
func requirePins(ctx context.Context) error {
	for _, pin := range pins {
		body, err := pin.read(ctx)
		if err != nil {
			return fmt.Errorf("egress wall: %w", err)
		}
		if answer := strings.TrimSpace(string(body)); answer != pin.want {
			return fmt.Errorf("egress wall: the %s %s's %s=%s, want %s — %s", pin.side, pin.kind, pin.sysctl, answer, pin.want, pin.because)
		}
	}
	return nil
}

// read answers the pin's value from its side's netns.
func (p pin) read(ctx context.Context) ([]byte, error) {
	if p.side == childSide {
		return readChildSysctl(ctx, p.path)
	}
	return readParentSysctl(ctx, p.path)
}

// parentSysctl reads a procfs path from this mount.
func parentSysctl(_ context.Context, path string) ([]byte, error) {
	body, err := os.ReadFile(path) //nolint:gosec // the fixed procfs paths
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return body, nil
}

// childSysctl reads a procfs path from inside the child netns.
func childSysctl(ctx context.Context, path string) ([]byte, error) {
	//nolint:gosec // the fixed child read
	out, err := exec.CommandContext(ctx, "ip", "netns", "exec", netnsName, "cat", path).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ip netns exec %s cat %s: %w\n%s", netnsName, path, err, out)
	}
	return out, nil
}
