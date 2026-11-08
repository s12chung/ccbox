//go:build linux

package identity

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/cbpf"
)

func TestDropFilterNames_ResolveOnThisArch(t *testing.T) {
	prog, err := cbpf.RenderDenyList(dropFilterNames)

	require.NoError(t, err)
	// 4 program instructions plus one deny pair per name
	assert.Len(t, prog, 8*(4+2*len(dropFilterNames)))
	assert.Equal(t, uint32(unix.SECCOMP_RET_ALLOW), binary.NativeEndian.Uint32(prog[len(prog)-4:]))
}

func TestDropFilterNames_LoadsAndStacks(t *testing.T) {
	setpriv, err := exec.LookPath("setpriv")
	if err != nil {
		t.Skip("setpriv not found")
	}
	base, ok := seccompFilterCount(t)
	if !ok {
		t.Skip("kernel does not report Seccomp_filters")
	}

	prog, err := cbpf.RenderDenyList(dropFilterNames)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "drop.bpf")
	require.NoError(t, os.WriteFile(path, prog, 0o400))

	//nolint:gosec // the test's own binary choice of setpriv, sh and the fixed script
	out, err := exec.CommandContext(t.Context(), setpriv, "--no-new-privs", "--seccomp-filter="+path,
		"--", "sh", "-c", "grep Seccomp /proc/self/status").CombinedOutput()

	require.NoError(t, err, "%s", out)
	assert.Contains(t, string(out), "Seccomp:\t2")
	assert.Contains(t, string(out), "Seccomp_filters:\t"+strconv.Itoa(base+1))
}

// seccompFilterCount reads this process's stacked-filter count — absent on
// kernels before 5.9.
func seccompFilterCount(t *testing.T) (int, bool) {
	t.Helper()
	st, err := os.ReadFile("/proc/self/status")
	require.NoError(t, err)
	for line := range strings.SplitSeq(string(st), "\n") {
		if count, found := strings.CutPrefix(line, "Seccomp_filters:\t"); found {
			n, err := strconv.Atoi(strings.TrimSpace(count))
			require.NoError(t, err)
			return n, true
		}
	}
	return 0, false
}
