// Package proxy is the egress wall's in-container mechanism: a pinned child
// netns whose only route out leads through tinyproxy — built at boot, proven by
// the proxy child, entered by every session through the drop chain.
package proxy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/identity"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/cmdgen"
)

// The egress wall's pinned child netns: the fixed name every entry aims at —
// the build's pin, the sessions' nsenter, the drop's proof.
const (
	dropUser = "ccbox" // the image's unprivileged user (1000)

	netnsName  = "egress-wall"
	netnsPath  = "/run/netns/" + netnsName
	nsenterBin = "nsenter"
	unshareBin = "unshare"
)

// DropChain wraps argv so it runs inside the egress wall.
func DropChain(argv ...string) []string {
	// entering the netns and creating the pidns need root, so both come before the drop
	cross := []string{nsenterBin, "--net=" + netnsPath, "--"}
	isolate := []string{
		unshareBin,
		"--pid", "--fork", // argv runs as the new namespace's first process — its /proc shows it alone
		"--mount-proc",          // the fresh procfs over /proc — the container's shows every session
		"--ipc",                 // fresh SysV IPC and mqueues — a same-uid channel the pidns can't cover
		"--propagation=private", // the session's mounts don't propagate to or from the host
		"--kill-child",          // killing the exec kills the session with it
		"--",
	}
	return slices.Concat(cross, isolate, identity.Drop(dropUser, argv...))
}

// Proxied reports the proxy env's presence — the env vars exist only when the
// egress wall does (`ccbox --no-proxy` runs without them).
func Proxied() bool { return os.Getenv(proxyEnv) != "" }

// NSIsolatedCmd builds the DropChain command that re-runs this binary inside
// the pinned netns, wired to the caller's stdio.
func NSIsolatedCmd(ctx context.Context, argv ...string) (*exec.Cmd, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	chain := DropChain(append([]string{self}, argv...)...)
	return cmdgen.Stdio(ctx, chain[0], chain[1:]...), nil
}

// SameNetNS reports whether this process already sits in the pinned child netns.
func SameNetNS() bool {
	self, err := os.Stat("/proc/self/ns/net")
	if err != nil {
		return false
	}
	pinned, err := os.Stat(netnsPath)
	return err == nil && os.SameFile(self, pinned)
}

// RequireNetNS proves residence in the pinned child netns — the egress wall's half of
// a drop's proof.
func RequireNetNS() error {
	if !SameNetNS() {
		return fmt.Errorf("not in the egress wall's netns (%s)", netnsPath)
	}
	return nil
}
