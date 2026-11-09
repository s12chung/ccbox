package proxy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/tinyproxy"
)

// The egress wall's fixed /30 — parent: tinyproxy's listen, child: the
// workload's address. Literal: the child netns resolves no names, no shared loopback.
const (
	childIP = "169.254.100.2"

	parentVeth = "proxy-parent"
	childVeth  = "proxy-child"

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

type (
	// Spawn builds a fresh, unstarted process command.
	Spawn func(ctx context.Context) (*exec.Cmd, error)
	// Ready awaits the forked child's readiness, owning its cleanup on failure.
	Ready func(ctx context.Context, cmd *exec.Cmd) error
	// PID answers the started process's pid, live across respawns.
	PID func() int
)

// CreateNetwork brings the egress wall up: the network, tinyproxy through
// start, and its allowlist watch — in order, failing closed.
func CreateNetwork(ctx context.Context, start func(ctx context.Context, spawn Spawn, ready Ready) (PID, error)) error {
	if err := buildNetns(ctx); err != nil {
		return err
	}
	pid, err := start(ctx, tinyproxy.Cmd, tinyproxy.AwaitListen)
	if err != nil {
		return err
	}
	return tinyproxy.WatchAllowlist(ctx, pid)
}

// buildNetns builds the egress wall's network: pure iproute2, no iptables —
// forwarding pinned off in both netns and rp_filter pinned strict on both
// cable ends.
func buildNetns(ctx context.Context) error {
	for _, args := range ipArgs {
		if err := runIP(ctx, args...); err != nil {
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

// pin is one sysctl the egress wall holds: the value each side must answer.
type pin struct {
	sysctl     string
	kind       string // completes the error subject — the side's netns or its veth
	parentPath string // this mount
	childPath  string // the same tree, from inside the child netns
	want       string
	because    string
}

// pins is the assert table, in order: forwarding first, then the cable ends.
var pins = []pin{
	// the child can reach the parent's own IPs directly — that's local
	// delivery, not forwarding; ip_forward=0 doesn't stop it, and only the
	// boot's own code could ever listen there
	{
		sysctl:     "net.ipv4.ip_forward",
		kind:       "netns",
		parentPath: ipForwardPath,
		childPath:  ipForwardPath,
		want:       "0",
		because:    "the child's traffic must blackhole, not forward",
	},
	{
		sysctl:     "net.ipv4.conf.rp_filter",
		kind:       "veth",
		parentPath: parentRPFilterPath,
		childPath:  childRPFilterPath,
		want:       "1",
		because:    "spoofed-source replies would leave unallowlisted",
	},
}

// requirePins asserts the pins hold — the child's only way out is the
// parent-local tinyproxy.
func requirePins(ctx context.Context) error {
	for _, pin := range pins {
		for _, side := range []struct {
			name string
			read func(context.Context) ([]byte, error)
		}{
			{"parent", pin.readParent},
			{"child", pin.readChild},
		} {
			body, err := side.read(ctx)
			if err != nil {
				return fmt.Errorf("egress wall: %w", err)
			}
			if answer := strings.TrimSpace(string(body)); answer != pin.want {
				return fmt.Errorf("egress wall: the %s %s's %s=%s, want %s — %s", side.name, pin.kind, pin.sysctl, answer, pin.want, pin.because)
			}
		}
	}
	return nil
}

// readParent answers the pin's value from this mount.
func (p pin) readParent(ctx context.Context) ([]byte, error) {
	return readParentSysctl(ctx, p.parentPath)
}

// readChild answers the pin's value from inside the child netns.
func (p pin) readChild(ctx context.Context) ([]byte, error) {
	return readChildSysctl(ctx, p.childPath)
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
