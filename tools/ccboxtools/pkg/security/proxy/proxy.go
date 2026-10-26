// Package proxy is the egress wall's in-container mechanism: a pinned child
// netns whose only route out leads through tinyproxy — built at boot, proven by
// the proxy child, entered by every exec session before its drop.
package proxy

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/cmdgen"
)

// The egress wall's pinned child netns: the fixed name every entry aims at —
// the build's pin, the sessions' nsenter, the drop's proof.
const (
	netnsName  = "egress-wall"
	netnsPath  = "/run/netns/" + netnsName
	nsenterBin = "nsenter"
)

// Proxied reports the proxy env's presence — the env vars exist only when the
// egress wall does (`ccbox --no-proxy` runs without them).
func Proxied() bool { return os.Getenv(proxyEnv) != "" }

// NSIsolatedCmd builds the command that re-runs this binary inside the pinned
// child netns, wired to the caller's stdio.
func NSIsolatedCmd(ctx context.Context, argv ...string) (*exec.Cmd, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return cmdgen.Stdio(ctx, nsenterBin, append([]string{"--net=" + netnsPath, "--", self}, argv...)...), nil
}

// NSIsolatedExec runs NSIsolatedCmd's command to completion, wired to the
// caller's stdio — the child's exit code is ours: a signal carries no code
// (−1), only its failure.
func NSIsolatedExec(ctx context.Context, argv ...string) error {
	inner, err := NSIsolatedCmd(ctx, argv...)
	if err != nil {
		return err
	}
	err = inner.Run()
	if inner.ProcessState == nil {
		return err // never started: the session's failure, not the argv's
	}
	os.Exit(max(inner.ProcessState.ExitCode(), 1))
	return nil
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
