package proxy

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/identity"
)

func TestDropChain(t *testing.T) {
	// the crossings — netns, pidns, ipcns — then the one-way drop, then the target: each
	// row a fixed argv the arrival proof depends on
	assert.Equal(t,
		[]string{
			"nsenter", "--net=" + netnsPath, "--",
			"unshare",
			"--pid", "--fork", "--mount-proc", "--ipc", "--propagation=private", "--kill-child",
			"--",
			"setpriv",
			"--reuid=ccbox", "--regid=ccbox", "--clear-groups",
			"--inh-caps=-all", "--ambient-caps=-all", "--bounding-set=-all",
			"--seccomp-filter=" + identity.DropFilterPath,
			"--no-new-privs", "--",
			"/usr/local/bin/ccboxtools", "proxy", "child",
		},
		DropChain("/usr/local/bin/ccboxtools", "proxy", "child"),
	)

	// a second call must not bleed argv into the shared drop args
	first := DropChain("one")
	assert.Equal(t, []string{"two"}, DropChain("two")[len(first)-1:], "argv aliases the drop")
}

func TestNSIsolatedCmd(t *testing.T) {
	cmd, err := NSIsolatedCmd(t.Context(), "exec", "--", "bash")
	require.NoError(t, err)

	assert.Contains(t, cmd.Path, nsenterBin) // LookPath resolves at construction
	self, err := os.Executable()
	require.NoError(t, err)
	assert.Equal(t, DropChain(self, "exec", "--", "bash"), cmd.Args)
	assert.Same(t, os.Stdin, cmd.Stdin)
	assert.Same(t, os.Stdout, cmd.Stdout)
	assert.Same(t, os.Stderr, cmd.Stderr)
}

func TestSameNetNS(t *testing.T) {
	// the verdict is keyed to the environment — an egress-walled devbox sits in the
	// pinned netns, anything else doesn't — so compare against the files directly
	// (a missing procfs just means not in the netns, like SameNetNS reads it)
	self, selfErr := os.Stat("/proc/self/ns/net")
	pinned, pinnedErr := os.Stat(netnsPath)
	inNetns := selfErr == nil && pinnedErr == nil && os.SameFile(self, pinned)
	assert.Equal(t, inNetns, SameNetNS())
}

func TestRequireNetNS(t *testing.T) {
	// in an egress-walled devbox the test process itself sits in the pinned
	// netns: both verdicts are correct, keyed to the live environment
	if SameNetNS() {
		require.NoError(t, RequireNetNS())
		return
	}
	err := RequireNetNS()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in the egress wall's netns")
}
