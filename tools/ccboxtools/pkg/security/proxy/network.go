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

// The egress wall's fixed /30: the parent-side IP is tinyproxy's listen and
// the child's only way out; the child-side IP is the workload's only address.
// Literal — the child netns resolves no names, and its loopback is its own.
const (
	childIP = "169.254.100.2"

	parentVeth = "egress-wall-parent"
	childVeth  = "egress-wall-child"
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
	ipForwardPath = "/proc/sys/net/ipv4/ip_forward"
)

// The start contract's halves: the spawn choice is this package's, the
// supervision — and its live pid answer — the caller's.
type (
	// Spawn starts a fresh process, answering with its command.
	Spawn func(ctx context.Context) (*exec.Cmd, error)

	// PID answers the started process's pid, live across respawns.
	PID func() int
)

// CreateNetwork brings the egress wall up: the network, tinyproxy through
// start, and its allowlist watch — in order, failing closed.
func CreateNetwork(ctx context.Context, start func(ctx context.Context, spawn Spawn) (PID, error)) error {
	if err := buildNetns(ctx); err != nil {
		return err
	}
	pid, err := start(ctx, tinyproxy.Start)
	if err != nil {
		return err
	}
	return tinyproxy.WatchAllowlist(ctx, pid)
}

// buildNetns builds the egress wall's network: pure iproute2 — no iptables, no
// ip_forward.
func buildNetns(ctx context.Context) error {
	for _, args := range ipArgs {
		if err := runIP(ctx, args...); err != nil {
			return fmt.Errorf("egress wall: %w", err)
		}
	}
	return requireNoForwarding()
}

// ipArgs is the egress wall's build table: one `ip` invocation per row, run
// in order — the order is load-bearing.
//
//nolint:goconst // the rows read as the shell commands they run; named tokens would obscure them
var ipArgs = [][]string{
	// our end: the pin, the cable, its address, raised — the pin's bind mount
	// keeps the netns alive past the build
	{"netns", "add", netnsName},
	{"link", "add", parentVeth, "type", "veth", "peer", "name", childVeth},
	{"addr", "add", pkginfo.ProxyAddr + "/30", "dev", parentVeth},
	{"link", "set", parentVeth, "up"},

	// our end complete — hand the child end across
	{"link", "set", childVeth, "netns", netnsName},

	// their end, from inside: the room's loopback, then its end of the cable
	{"-n", netnsName, "link", "set", "lo", "up"},
	{"-n", netnsName, "addr", "add", childIP + "/30", "dev", childVeth},
	{"-n", netnsName, "link", "set", childVeth, "up"},

	// the exit sign, last — once the road exists
	{"-n", netnsName, "route", "replace", "default", "via", pkginfo.ProxyAddr},
	// the child carries no IPv6 route: with IPv6 off it has no addresses, so
	// nothing to route and nowhere to reach
	{"netns", "exec", netnsName, "sysctl", "-qw", "net.ipv6.conf.all.disable_ipv6=1"},
	{"netns", "exec", netnsName, "sysctl", "-qw", "net.ipv6.conf.default.disable_ipv6=1"},
}

// requireNoForwarding asserts the parent netns never forwards — the child's
// only way out is the parent-local tinyproxy.
func requireNoForwarding() error {
	body, err := os.ReadFile(ipForwardPath) //nolint:gosec // the fixed procfs path
	if err != nil {
		return fmt.Errorf("egress wall: read %s: %w", ipForwardPath, err)
	}
	if v := strings.TrimSpace(string(body)); v != "0" {
		return fmt.Errorf("egress wall: net.ipv4.ip_forward=%s, want 0 — the child's traffic must blackhole, not forward", v)
	}
	return nil
}
